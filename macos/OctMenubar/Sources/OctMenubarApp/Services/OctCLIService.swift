import Darwin
import Foundation

struct OctCLIService {
    let executableURL: URL
    let searchedPaths: [String]
    var refreshInterval: TimeInterval = 60
    var processTimeout: TimeInterval = 20
    /// Grace between SIGTERM and SIGKILL, and the poll window for settling
    /// the exit state. SIGTERM can sit pending in an uninterruptible child or
    /// be trapped by a wrapper, so the watchdog always escalates.
    static let exitGrace: TimeInterval = 2
    /// Wall-clock budget of one fetch: the process timeout plus the SIGKILL
    /// and exit-settle grace windows plus headroom. Consumers that gate
    /// refreshes (UsageViewModel) use it as their completion deadline so a
    /// wedged subprocess can never wedge the menubar.
    var refreshDeadline: TimeInterval

    init(
        executableURL: URL? = nil,
        refreshInterval: TimeInterval = 60,
        processTimeout: TimeInterval = 20,
        refreshDeadline: TimeInterval? = nil,
        environment: [String: String] = ProcessInfo.processInfo.environment,
        currentDirectoryURL: URL = URL(fileURLWithPath: FileManager.default.currentDirectoryPath),
        processExecutableURL: URL = URL(fileURLWithPath: CommandLine.arguments[0])
    ) {
        self.refreshInterval = refreshInterval
        self.processTimeout = processTimeout
        self.refreshDeadline = refreshDeadline ?? processTimeout + Self.exitGrace * 2 + 6
        if let executableURL {
            self.executableURL = executableURL
            self.searchedPaths = [executableURL.path]
        } else {
            let resolution = Self.resolveExecutable(
                environment: environment,
                currentDirectoryURL: currentDirectoryURL,
                processExecutableURL: processExecutableURL
            )
            self.executableURL = resolution.url
            self.searchedPaths = resolution.searchedPaths
        }
    }

    /// Builds a snapshot from usage output plus a (possibly cached)
    /// configuration. The configuration is injected by the caller — the
    /// shared ConfigurationStore — instead of re-reading it on every refresh.
    func fetchUsageSnapshot(
        configuration: ConfigurationSnapshot?,
        now: Date = Date()
    ) async throws -> UsageSnapshot {
        let titleMode = configuration?.menubarTitleMode ?? .oct
        let refreshInterval = configuration?.refreshInterval ?? refreshInterval
        let output = try await runAndCapture(arguments: ["usage", "--json"])
        let data = Data(output.utf8)
        let response = try JSONDecoder().decode(UsageResponse.self, from: data)
        return UsageSnapshot.from(
            response: response,
            refreshDate: now,
            refreshInterval: refreshInterval,
            titleMode: titleMode
        )
    }

    /// - Parameter probeVersions: also ask the oct CLI to probe each tool's
    ///   installed version. The probe shells out per tool, so only surfaces
    ///   that show versions (the settings load/save) should pay for it — the
    ///   popover reload path keeps the plain snapshot.
    func fetchConfigurationSnapshot(probeVersions: Bool = false) async throws -> ConfigurationSnapshot {
        var arguments = ["config", "list", "--json"]
        if probeVersions {
            arguments.append("--probe-versions")
        }
        let output = try await runAndCapture(arguments: arguments)
        let data = Data(output.utf8)
        return try JSONDecoder().decode(ConfigurationSnapshot.self, from: data)
    }

    /// Saves via the modern --payload flag. When the installed oct binary is
    /// older and rejects the flag, retries once with the legacy --json
    /// spelling — only for that positively-identified flag-parse failure;
    /// every other error surfaces unchanged (a generic retry could repeat an
    /// already-applied change or mask the real problem).
    func saveConfiguration(_ payload: ConfigurationUpdatePayload) async throws {
        let data = try JSONEncoder().encode(payload)
        guard let json = String(data: data, encoding: .utf8) else {
            throw OctCLIServiceError.encodingFailed
        }
        do {
            _ = try await runProcess(executableURL: executableURL, arguments: ["config", "update", "--payload", json])
        } catch let error as OctCLIServiceError {
            if case .nonZeroExit(_, let stderr) = error, Self.isUnknownFlagError(stderr) {
                _ = try await runProcess(executableURL: executableURL, arguments: ["config", "update", "--json", json])
                return
            }
            throw error
        }
    }

    static func isUnknownFlagError(_ stderr: String) -> Bool {
        let lowered = stderr.lowercased()
        return lowered.contains("unknown flag: --payload")
            || lowered.contains("flag provided but not defined: -payload")
    }

    /// Restarts the menubar helper by handing stop→start to a detached shell:
    /// `oct menubar stop` SIGTERMs every helper instance (this one included),
    /// then `oct menubar --daemon` starts a fresh one. The sequence must
    /// outlive this terminating app, so it runs unmanaged — no pipes or
    /// watchdog, unlike `runProcess`, which would die with the app.
    func restartHelperDetached() throws {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/zsh")
        process.arguments = ["-c", Self.helperRestartScript(octPath: executableURL.path)]
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        try process.run()
    }

    static func helperRestartScript(octPath: String) -> String {
        let oct = shellQuote(octPath)
        return "\(oct) menubar stop; sleep 0.5; \(oct) menubar --daemon"
    }

    func run(action: OctMenubarAction) async throws {
        switch action {
        case .openUsage:
            try await runInTerminal(arguments: ["usage"])
        case .openMonitor:
            try await runInTerminal(arguments: ["monitor", "--once"])
        case .runSessionRefresh:
            try await runInTerminal(arguments: ["session-refresh"])
        case .runAlertCheck:
            try await runInTerminal(arguments: ["usage", "--notify"])
        case .runAgentUpdate:
            try await runInTerminal(arguments: ["agent-update"])
        }
    }

    /// Arguments for the background agent update: `--json` streams NDJSON
    /// update events the app parses, and JSON mode never prompts — the
    /// app asked the user beforehand, so missing providers are either
    /// installed (`--install-missing`) or skipped (`--skip-missing`).
    /// `only` restricts the run to the named providers (CLI `--only`).
    static func agentUpdateArguments(installMissing: Bool, only: [String]? = nil) -> [String] {
        var arguments = ["agent-update", "--json", installMissing ? "--install-missing" : "--skip-missing"]
        if let only, !only.isEmpty {
            arguments.append("--only")
            arguments.append(contentsOf: only)
        }
        return arguments
    }

    /// The CLI caps its install phase at 10 minutes internally; the app-side
    /// watchdog only guards against a wedged process, so it sits above that.
    static let agentUpdateTimeout: TimeInterval = 12 * 60

    /// The check mode shells out to npm/brew outdated probes per provider but
    /// never installs anything; the CLI caps itself at 45 seconds, and this
    /// watchdog sits above that.
    static let agentUpdateCheckTimeout: TimeInterval = 90

    /// Runs `oct agent-update --json` in the background and streams each
    /// decoded update event to `onEvent` from a pipe-drain thread (hop to the
    /// main actor at the call site). Returns when the process exits.
    /// Infrastructure failures — missing binary, launch failure, watchdog
    /// timeout, undeterminable fate — throw; per-tool failures arrive as events.
    func runAgentUpdate(
        installMissing: Bool = false,
        only: [String]? = nil,
        timeout: TimeInterval = OctCLIService.agentUpdateTimeout,
        eofGrace: TimeInterval = 120,
        onEvent: @escaping @Sendable (AgentUpdateEvent) -> Void
    ) async throws -> AgentUpdateRunResult {
        try await runEventStream(
            arguments: Self.agentUpdateArguments(installMissing: installMissing, only: only),
            timeout: timeout,
            eofGrace: eofGrace,
            onEvent: onEvent
        )
    }

    /// Runs `oct agent-update --check --json` and streams the version-check
    /// events (check_start / tool_check / check_done) without updating
    /// anything. Never prompts, so no stdin handling beyond the null device.
    func runAgentUpdateCheck(
        timeout: TimeInterval = OctCLIService.agentUpdateCheckTimeout,
        eofGrace: TimeInterval = 30,
        onEvent: @escaping @Sendable (AgentUpdateEvent) -> Void
    ) async throws -> AgentUpdateRunResult {
        try await runEventStream(
            arguments: ["agent-update", "--check", "--json"],
            timeout: timeout,
            eofGrace: eofGrace,
            onEvent: onEvent
        )
    }

    /// Shared streaming core for the agent-update run and check modes: spawns
    /// `oct` with the given arguments, drains stdout line-by-line into decoded
    /// events, and applies the same two-stage watchdog as runProcess.
    private func runEventStream(
        arguments: [String],
        timeout: TimeInterval,
        eofGrace: TimeInterval,
        onEvent: @escaping @Sendable (AgentUpdateEvent) -> Void
    ) async throws -> AgentUpdateRunResult {
        let fileManager = FileManager.default
        guard fileManager.isExecutableFile(atPath: executableURL.path) else {
            throw OctCLIServiceError.missingExecutable(path: executableURL.path, searchedPaths: searchedPaths)
        }

        let process = Process()
        process.executableURL = executableURL
        process.arguments = arguments

        let stdout = Pipe()
        let stderr = Pipe()
        process.standardOutput = stdout
        process.standardError = stderr
        // stdin pinned to the null device: --json never prompts, and an open
        // but silent pipe could stall a prompted run indefinitely.
        process.standardInput = FileHandle.nullDevice

        let stderrBuffer = LockedBuffer()
        let lineBuffer = LockedLineBuffer()
        let timedOut = AtomicFlag()

        // Drain stdout line-by-line as data arrives so install output beyond
        // the pipe buffer capacity can never block the child on write, and
        // each event reaches the UI the moment its line completes.
        let pipesEOF = DispatchGroup()
        pipesEOF.enter()
        pipesEOF.enter()
        stdout.fileHandleForReading.readabilityHandler = { handle in
            let chunk = handle.availableData
            if chunk.isEmpty {
                handle.readabilityHandler = nil
                for line in lineBuffer.flush() {
                    if let event = AgentUpdateEvent.decode(line: line) {
                        onEvent(event)
                    }
                }
                pipesEOF.leave()
            } else {
                for line in lineBuffer.append(chunk) {
                    if let event = AgentUpdateEvent.decode(line: line) {
                        onEvent(event)
                    }
                }
            }
        }
        stderr.fileHandleForReading.readabilityHandler = { handle in
            let chunk = handle.availableData
            if chunk.isEmpty {
                handle.readabilityHandler = nil
                pipesEOF.leave()
            } else {
                stderrBuffer.append(chunk)
            }
        }

        do {
            try process.run()
        } catch {
            stdout.fileHandleForReading.readabilityHandler = nil
            stderr.fileHandleForReading.readabilityHandler = nil
            throw OctCLIServiceError.launchFailed(path: executableURL.path, underlying: error)
        }

        // Same two-stage watchdog as runProcess, scaled to the update budget:
        // SIGTERM at `timeout`, SIGKILL one grace window later.
        let terminateWorkItem = DispatchWorkItem {
            timedOut.set()
            if process.isRunning {
                process.terminate()
            }
        }
        let killWorkItem = DispatchWorkItem {
            timedOut.set()
            if process.isRunning {
                kill(process.processIdentifier, SIGKILL)
            }
        }
        DispatchQueue.global().asyncAfter(deadline: .now() + timeout, execute: terminateWorkItem)
        DispatchQueue.global().asyncAfter(deadline: .now() + timeout + Self.exitGrace, execute: killWorkItem)
        defer {
            terminateWorkItem.cancel()
            killWorkItem.cancel()
        }

        // Both pipes closed => the child (and anything holding its stdout) is
        // gone. Bounded well past the watchdog (a grandchild inheriting the
        // pipe could otherwise hold it open indefinitely); `eofGrace` exists
        // so tests can tighten the bound.
        let eofDeadline = timeout + Self.exitGrace * 2 + eofGrace
        let pipesClosed = await waitForPipesEOF(pipesEOF, deadline: eofDeadline)

        guard let exitStatus = await reapExitStatus(process, grace: Self.exitGrace) else {
            throw OctCLIServiceError.timeout(path: executableURL.path, timeout: timeout)
        }
        if timedOut.isSet {
            throw OctCLIServiceError.timeout(path: executableURL.path, timeout: timeout)
        }
        guard pipesClosed else {
            throw OctCLIServiceError.timeout(path: executableURL.path, timeout: eofDeadline)
        }

        return AgentUpdateRunResult(
            exitStatus: exitStatus,
            stderr: String(data: stderrBuffer.snapshot(), encoding: .utf8) ?? ""
        )
    }

    static func resolveExecutable(
        environment: [String: String],
        currentDirectoryURL: URL,
        processExecutableURL: URL
    ) -> (url: URL, searchedPaths: [String]) {
        let fileManager = FileManager.default
        let candidates = candidateExecutableURLs(
            environment: environment,
            currentDirectoryURL: currentDirectoryURL,
            processExecutableURL: processExecutableURL
        )

        var searchedPaths: [String] = []
        for candidate in candidates {
            let standardized = candidate.standardizedFileURL
            let path = standardized.path
            searchedPaths.append(path)
            if fileManager.isExecutableFile(atPath: path) {
                return (standardized, searchedPaths)
            }
        }

        return (candidates.first?.standardizedFileURL ?? currentDirectoryURL.appendingPathComponent("oct"), searchedPaths)
    }

    static func candidateExecutableURLs(
        environment: [String: String],
        currentDirectoryURL: URL,
        processExecutableURL: URL
    ) -> [URL] {
        var candidates: [URL] = []
        var seen: Set<String> = []

        func append(_ url: URL?) {
            guard let url else { return }
            let standardized = url.standardizedFileURL
            let path = standardized.path
            guard seen.insert(path).inserted else { return }
            candidates.append(standardized)
        }

        if let explicit = environment["OCT_MENUBAR_OCT_PATH"], !explicit.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            append(URL(fileURLWithPath: explicit))
        }

        let baseDirectories = [
            currentDirectoryURL,
            processExecutableURL.deletingLastPathComponent(),
        ]

        for base in baseDirectories {
            var cursor = base.standardizedFileURL
            for _ in 0..<6 {
                append(cursor.appendingPathComponent("oct"))
                let parent = cursor.deletingLastPathComponent()
                if parent.path == cursor.path { break }
                cursor = parent
            }
        }

        if let rawPath = environment["PATH"] {
            for component in rawPath.split(separator: ":") where !component.isEmpty {
                append(URL(fileURLWithPath: String(component)).appendingPathComponent("oct"))
            }
        }

        return candidates
    }

    private func runAndCapture(arguments: [String]) async throws -> String {
        let result = try await runProcess(executableURL: executableURL, arguments: arguments)
        return result.stdout
    }

    private func runInTerminal(arguments: [String]) async throws {
        let launcherURL = FileManager.default.temporaryDirectory
            .appendingPathComponent("oct-menubar-\(UUID().uuidString)")
            .appendingPathExtension("command")
        let script = """
        #!/bin/zsh
        \(buildShellCommand(arguments: arguments))
        status=$?
        echo
        echo "[oct menubar] exit status: $status"
        echo "Press any key to close..."
        read -k 1
        exit $status
        """
        try script.write(to: launcherURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: launcherURL.path)

        _ = try await runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/open"),
            arguments: ["-a", "Terminal", launcherURL.path],
            requireManagedExecutable: false
        )

        // Terminal may not have exec'd the launcher yet when `open` returns;
        // remove it after a grace period instead of leaking one file per
        // action into the shared temp directory.
        DispatchQueue.global().asyncAfter(deadline: .now() + 60) {
            try? FileManager.default.removeItem(at: launcherURL)
        }
    }

    private func buildShellCommand(arguments: [String]) -> String {
        ([Self.shellQuote(executableURL.path)] + arguments.map(Self.shellQuote)).joined(separator: " ")
    }

    /// Runs a process off the caller's thread and collects its output without
    /// deadlock: pipes are drained continuously from the moment the process
    /// starts (so output beyond the pipe buffer capacity cannot block the
    /// child on write), and every wait is deadline-bounded. The two-stage
    /// watchdog sends SIGTERM at `processTimeout` and SIGKILL after
    /// `exitGrace`; the pipe-EOF wait and the exit settle are polled against
    /// deadlines because both can stall indefinitely — a grandchild inheriting
    /// the pipe holds it open past termination, and Foundation's termination
    /// bookkeeping can miss a child's exit entirely, leaving waitUntilExit
    /// blocked forever on a dead child (observed in the field as a menubar
    /// that never refreshed again). Every stall path fails the refresh
    /// instead of hanging.
    private func runProcess(
        executableURL: URL,
        arguments: [String],
        requireManagedExecutable: Bool = true
    ) async throws -> ProcessOutput {
        let fileManager = FileManager.default
        if requireManagedExecutable && !fileManager.isExecutableFile(atPath: executableURL.path) {
            throw OctCLIServiceError.missingExecutable(path: executableURL.path, searchedPaths: searchedPaths)
        }

        let process = Process()
        process.executableURL = executableURL
        process.arguments = arguments

        let stdout = Pipe()
        let stderr = Pipe()
        process.standardOutput = stdout
        process.standardError = stderr

        // Reference-typed, lock-guarded buffers: readabilityHandler closures
        // append concurrently, and Swift 6 forbids mutating captured vars.
        let stdoutBuffer = LockedBuffer()
        let stderrBuffer = LockedBuffer()
        let timedOut = AtomicFlag()

        // Drain both pipes as data arrives; each handler removes itself at
        // EOF. The group completes only when both pipes are exhausted.
        let pipesEOF = DispatchGroup()
        pipesEOF.enter()
        pipesEOF.enter()
        stdout.fileHandleForReading.readabilityHandler = { handle in
            let chunk = handle.availableData
            if chunk.isEmpty {
                handle.readabilityHandler = nil
                pipesEOF.leave()
            } else {
                stdoutBuffer.append(chunk)
            }
        }
        stderr.fileHandleForReading.readabilityHandler = { handle in
            let chunk = handle.availableData
            if chunk.isEmpty {
                handle.readabilityHandler = nil
                pipesEOF.leave()
            } else {
                stderrBuffer.append(chunk)
            }
        }

        do {
            try process.run()
        } catch {
            stdout.fileHandleForReading.readabilityHandler = nil
            stderr.fileHandleForReading.readabilityHandler = nil
            throw OctCLIServiceError.launchFailed(path: executableURL.path, underlying: error)
        }

        let terminateWorkItem = DispatchWorkItem {
            timedOut.set()
            if process.isRunning {
                process.terminate()
            }
        }
        let killWorkItem = DispatchWorkItem {
            timedOut.set()
            if process.isRunning {
                // Process exposes no SIGKILL API; escalate via the kernel
                // directly because terminate() (SIGTERM) can sit pending in
                // an uninterruptible child or be trapped by a wrapper.
                kill(process.processIdentifier, SIGKILL)
            }
        }
        DispatchQueue.global().asyncAfter(deadline: .now() + processTimeout, execute: terminateWorkItem)
        DispatchQueue.global().asyncAfter(deadline: .now() + processTimeout + Self.exitGrace, execute: killWorkItem)
        defer {
            terminateWorkItem.cancel()
            killWorkItem.cancel()
        }

        // Both pipes closed => the child and anything holding its stdout or
        // stderr is gone. Bounded: a grandchild inheriting the pipe could
        // otherwise hold it open well past the watchdog.
        let pipesClosed = await waitForPipesEOF(pipesEOF, deadline: refreshDeadline)

        let stdoutText = String(data: stdoutBuffer.snapshot(), encoding: .utf8) ?? ""
        let stderrText = String(data: stderrBuffer.snapshot(), encoding: .utf8) ?? ""

        guard let exitStatus = await reapExitStatus(process, grace: Self.exitGrace) else {
            // The child's fate is unknown even after SIGKILL and both grace
            // windows — fail this refresh instead of waiting forever.
            throw OctCLIServiceError.timeout(path: executableURL.path, timeout: refreshDeadline)
        }
        if timedOut.isSet {
            throw OctCLIServiceError.timeout(path: executableURL.path, timeout: processTimeout)
        }
        guard pipesClosed else {
            // EOF never arrived: something outside the child still holds the
            // pipes, so the output cannot be trusted as complete.
            throw OctCLIServiceError.timeout(path: executableURL.path, timeout: refreshDeadline)
        }
        guard exitStatus == 0 else {
            throw OctCLIServiceError.nonZeroExit(
                status: exitStatus,
                stderr: stderrText.trimmingCharacters(in: .whitespacesAndNewlines)
            )
        }

        return ProcessOutput(stdout: stdoutText, stderr: stderrText)
    }

    /// Returns true when both pipes hit EOF; false when the deadline elapsed
    /// first (something outside the child — typically a grandchild — still
    /// holds the pipes).
    private func waitForPipesEOF(_ pipesEOF: DispatchGroup, deadline: TimeInterval) async -> Bool {
        let gate = OneShotGate()
        pipesEOF.notify(queue: .global()) { gate.finish(pipesClosed: true) }
        DispatchQueue.global().asyncAfter(deadline: .now() + deadline) { gate.finish(pipesClosed: false) }
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            gate.arm(continuation)
        }
        return gate.didPipesClose
    }

    /// Collects the child's exit status without Foundation: waitpid polls the
    /// kernel directly and cannot hang, while waitUntilExit can block forever
    /// on a dead child when Foundation's launch handshake races a fast exit
    /// (observed in the field; reproduced in tests). A live child gets a
    /// grace window, then SIGKILL, then one more bounded reap. An ECHILD
    /// result means Foundation already reaped the child, whose
    /// terminationStatus is then the authoritative outcome. Returns nil when
    /// the status cannot be determined within the budget. The wait status is
    /// decoded by hand (sys/wait.h's WIFEXITED & friends are function-like
    /// macros Swift cannot import): low 7 bits carry the signal, high 8 the
    /// exit code.
    private func reapExitStatus(_ process: Process, grace: TimeInterval) async -> Int32? {
        let pid = process.processIdentifier
        guard pid > 0 else { return nil }
        for phase in 0..<2 {
            if phase == 1 {
                kill(pid, SIGKILL)
            }
            var status: Int32 = 0
            var deadline = Date().addingTimeInterval(grace)
            while Date() < deadline {
                let result = waitpid(pid, &status, WNOHANG)
                if result == pid {
                    let signal = status & 0x7F
                    if signal == 0 {
                        return (status >> 8) & 0xFF
                    }
                    if signal != 0x7F {
                        return 128 + signal
                    }
                    return nil
                }
                if result == -1 && errno == ECHILD {
                    return process.terminationStatus
                }
                try? await Task.sleep(nanoseconds: 10_000_000)
            }
        }
        return nil
    }

    static func shellQuote(_ value: String) -> String {
        if value.isEmpty { return "''" }
        return "'" + value.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }
}

/// Lock-guarded accumulator shared between pipe-reading callbacks and the
/// awaiting caller. @unchecked Sendable: all access goes through NSLock.
private final class LockedBuffer: @unchecked Sendable {
    private let lock = NSLock()
    private var data = Data()

    func append(_ chunk: Data) {
        lock.lock()
        data.append(chunk)
        lock.unlock()
    }

    func snapshot() -> Data {
        lock.lock()
        defer { lock.unlock() }
        return data
    }
}

/// Lock-guarded NDJSON line splitter for the streaming stdout reader: chunks
/// arrive on pipe threads and may split a line (or a multi-byte character)
/// mid-way, so bytes accumulate until a newline. The trailing partial line —
/// a crash can die mid-record — flushes at EOF.
private final class LockedLineBuffer: @unchecked Sendable {
    private let lock = NSLock()
    private var data = Data()

    /// Appends a chunk and returns every completed line (without newlines).
    func append(_ chunk: Data) -> [String] {
        lock.lock()
        data.append(chunk)
        var lines: [String] = []
        while let newline = data.firstIndex(of: UInt8(ascii: "\n")) {
            let lineData = data.subdata(in: data.startIndex..<newline)
            data.removeSubrange(data.startIndex...newline)
            if let line = String(data: lineData, encoding: .utf8), !line.isEmpty {
                lines.append(line)
            }
        }
        lock.unlock()
        return lines
    }

    /// Returns and clears trailing bytes that never saw a newline.
    func flush() -> [String] {
        lock.lock()
        defer { lock.unlock() }
        guard !data.isEmpty else { return [] }
        let line = String(data: data, encoding: .utf8)
        data = Data()
        return (line?.isEmpty == false) ? [line!] : []
    }
}

/// Lock-guarded boolean flag set from the timeout watchdog.
private final class AtomicFlag: @unchecked Sendable {
    private let lock = NSLock()
    private var value = false

    func set() {
        lock.lock()
        value = true
        lock.unlock()
    }

    var isSet: Bool {
        lock.lock()
        defer { lock.unlock() }
        return value
    }
}

/// Resumes exactly one continuation, from whichever waiter (pipe EOF or the
/// overall deadline) finishes first, and records the winner; a later fire is
/// a no-op. Lock-guarded: notify and the deadline fire on arbitrary
/// global-queue threads, and either may win the race against the arming call.
private final class OneShotGate: @unchecked Sendable {
    private let lock = NSLock()
    private var continuation: CheckedContinuation<Void, Never>?
    private var finished = false
    private var pipesClosed = false

    func finish(pipesClosed: Bool) {
        lock.lock()
        guard !finished else {
            lock.unlock()
            return
        }
        finished = true
        self.pipesClosed = pipesClosed
        let continuation = self.continuation
        self.continuation = nil
        lock.unlock()
        continuation?.resume()
    }

    func arm(_ continuation: CheckedContinuation<Void, Never>) {
        lock.lock()
        if finished {
            lock.unlock()
            continuation.resume()
            return
        }
        self.continuation = continuation
        lock.unlock()
    }

    var didPipesClose: Bool {
        lock.lock()
        defer { lock.unlock() }
        return pipesClosed
    }
}

struct ProcessOutput {
    let stdout: String
    let stderr: String
}

enum OctCLIServiceError: LocalizedError {
    case missingExecutable(path: String, searchedPaths: [String])
    case launchFailed(path: String, underlying: Error)
    case nonZeroExit(status: Int32, stderr: String)
    case timeout(path: String, timeout: TimeInterval)
    case encodingFailed

    var errorDescription: String? {
        switch self {
        case .missingExecutable(let path, let searchedPaths):
            let tried = searchedPaths.isEmpty ? path : searchedPaths.joined(separator: ", ")
            return "oct executable not found. Tried: \(tried)"
        case .launchFailed(let path, let underlying):
            return "failed to launch \(path): \(underlying.localizedDescription)"
        case .nonZeroExit(let status, let stderr):
            if stderr.isEmpty {
                return "oct exited with status \(status)"
            }
            return "oct exited with status \(status): \(stderr)"
        case .timeout(let path, let timeout):
            return "oct timed out after \(Int(timeout))s while running \(path)"
        case .encodingFailed:
            return "failed to encode configuration payload"
        }
    }
}
