import Foundation

struct OctCLIService {
    let executableURL: URL
    let searchedPaths: [String]
    var refreshInterval: TimeInterval = 60
    var processTimeout: TimeInterval = 20

    init(
        executableURL: URL? = nil,
        refreshInterval: TimeInterval = 60,
        processTimeout: TimeInterval = 20,
        environment: [String: String] = ProcessInfo.processInfo.environment,
        currentDirectoryURL: URL = URL(fileURLWithPath: FileManager.default.currentDirectoryPath),
        processExecutableURL: URL = URL(fileURLWithPath: CommandLine.arguments[0])
    ) {
        self.refreshInterval = refreshInterval
        self.processTimeout = processTimeout
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
    ) throws -> UsageSnapshot {
        let titleMode = configuration?.menubarTitleMode ?? .oct
        let usageDisplayMode = configuration?.usageDisplayMode ?? .remaining
        let refreshInterval = configuration?.refreshInterval ?? refreshInterval
        let output = try runAndCapture(arguments: ["usage", "--json"])
        let data = Data(output.utf8)
        let response = try JSONDecoder().decode(UsageResponse.self, from: data)
        return UsageSnapshot.from(
            response: response,
            refreshDate: now,
            refreshInterval: refreshInterval,
            titleMode: titleMode,
            usageDisplayMode: usageDisplayMode
        )
    }

    func fetchConfigurationSnapshot() async throws -> ConfigurationSnapshot {
        let output = try await runAndCapture(arguments: ["config", "list", "--json"])
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
        }
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
    }

    private func buildShellCommand(arguments: [String]) -> String {
        ([shellQuote(executableURL.path)] + arguments.map(shellQuote)).joined(separator: " ")
    }

    /// Runs a process off the caller's thread and collects its output without
    /// deadlock: pipes are drained continuously from the moment the process
    /// starts (so output beyond the pipe buffer capacity cannot block the
    /// child on write), and the call resolves only after the process has
    /// terminated AND both pipes reached EOF, so trailing output is never
    /// lost. A watchdog terminates the process at `processTimeout`; the
    /// continuation resumes exactly once.
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

        let lock = NSLock()
        var stdoutData = Data()
        var stderrData = Data()
        var didTimeOut = false

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
                lock.lock()
                stdoutData.append(chunk)
                lock.unlock()
            }
        }
        stderr.fileHandleForReading.readabilityHandler = { handle in
            let chunk = handle.availableData
            if chunk.isEmpty {
                handle.readabilityHandler = nil
                pipesEOF.leave()
            } else {
                lock.lock()
                stderrData.append(chunk)
                lock.unlock()
            }
        }

        do {
            try process.run()
        } catch {
            stdout.fileHandleForReading.readabilityHandler = nil
            stderr.fileHandleForReading.readabilityHandler = nil
            throw OctCLIServiceError.launchFailed(path: executableURL.path, underlying: error)
        }

        let timeoutWorkItem = DispatchWorkItem {
            lock.lock()
            didTimeOut = true
            lock.unlock()
            if process.isRunning {
                process.terminate()
            }
        }
        DispatchQueue.global().asyncAfter(
            deadline: .now() + processTimeout,
            execute: timeoutWorkItem
        )
        defer { timeoutWorkItem.cancel() }

        // Wait for both pipes to reach EOF (guaranteed after termination).
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            pipesEOF.notify(queue: .global()) {
                continuation.resume()
            }
        }
        process.waitUntilExit()

        lock.lock()
        let timedOut = didTimeOut
        let outData = stdoutData
        let errData = stderrData
        lock.unlock()

        if timedOut {
            throw OctCLIServiceError.timeout(path: executableURL.path, timeout: processTimeout)
        }

        let stdoutText = String(data: outData, encoding: .utf8) ?? ""
        let stderrText = String(data: errData, encoding: .utf8) ?? ""

        guard process.terminationStatus == 0 else {
            throw OctCLIServiceError.nonZeroExit(
                status: process.terminationStatus,
                stderr: stderrText.trimmingCharacters(in: .whitespacesAndNewlines)
            )
        }

        return ProcessOutput(stdout: stdoutText, stderr: stderrText)
    }

    private func shellQuote(_ value: String) -> String {
        if value.isEmpty { return "''" }
        return "'" + value.replacingOccurrences(of: "'", with: "'\\''") + "'"
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
