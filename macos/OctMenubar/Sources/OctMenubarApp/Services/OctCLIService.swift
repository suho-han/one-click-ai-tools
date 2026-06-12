import Foundation

struct OctCLIService {
    let executableURL: URL
    let searchedPaths: [String]
    var refreshInterval: TimeInterval = 60

    init(
        executableURL: URL? = nil,
        refreshInterval: TimeInterval = 60,
        environment: [String: String] = ProcessInfo.processInfo.environment,
        currentDirectoryURL: URL = URL(fileURLWithPath: FileManager.default.currentDirectoryPath),
        processExecutableURL: URL = URL(fileURLWithPath: CommandLine.arguments[0])
    ) {
        self.refreshInterval = refreshInterval
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

    func fetchUsageSnapshot(now: Date = Date()) throws -> UsageSnapshot {
        let output = try runAndCapture(arguments: ["usage", "--json"])
        let data = Data(output.utf8)
        let response = try JSONDecoder().decode(UsageResponse.self, from: data)
        return UsageSnapshot.from(response: response, refreshDate: now, refreshInterval: refreshInterval)
    }

    func run(action: OctMenubarAction) throws {
        switch action {
        case .openUsage:
            try runInTerminal(arguments: ["usage"])
        case .openMonitor:
            try runInTerminal(arguments: ["monitor", "--once"])
        case .runSessionRefresh:
            try runInTerminal(arguments: ["session-refresh"])
        case .runAlertCheck:
            try runInTerminal(arguments: ["usage", "--notify"])
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

    private func runAndCapture(arguments: [String]) throws -> String {
        let result = try runProcess(executableURL: executableURL, arguments: arguments)
        return result.stdout
    }

    private func runInTerminal(arguments: [String]) throws {
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

        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/open"),
            arguments: ["-a", "Terminal", launcherURL.path],
            requireManagedExecutable: false
        )
    }

    private func buildShellCommand(arguments: [String]) -> String {
        ([shellQuote(executableURL.path)] + arguments.map(shellQuote)).joined(separator: " ")
    }

    private func runProcess(
        executableURL: URL,
        arguments: [String],
        requireManagedExecutable: Bool = true
    ) throws -> ProcessOutput {
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

        do {
            try process.run()
        } catch {
            throw OctCLIServiceError.launchFailed(path: executableURL.path, underlying: error)
        }
        process.waitUntilExit()

        let stdoutData = stdout.fileHandleForReading.readDataToEndOfFile()
        let stderrData = stderr.fileHandleForReading.readDataToEndOfFile()
        let stdoutText = String(data: stdoutData, encoding: .utf8) ?? ""
        let stderrText = String(data: stderrData, encoding: .utf8) ?? ""

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
        }
    }
}
