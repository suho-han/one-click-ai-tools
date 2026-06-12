import Foundation

struct OctCLIService {
    var executableURL: URL = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
        .appendingPathComponent("oct")
    var refreshInterval: TimeInterval = 60

    func fetchUsageSnapshot(now: Date = Date()) throws -> UsageSnapshot {
        let output = try runAndCapture(arguments: ["usage", "--json"])
        let data = Data(output.utf8)
        let response = try JSONDecoder().decode(UsageResponse.self, from: data)
        return UsageSnapshot.from(response: response, refreshDate: now, refreshInterval: refreshInterval)
    }

    func run(action: OctMenubarAction) throws {
        switch action {
        case .openUsage:
            try runDetached(arguments: ["usage"])
        case .openMonitor:
            try runDetached(arguments: ["monitor", "--once"])
        case .runSessionRefresh:
            try runDetached(arguments: ["session-refresh"])
        case .runAlertCheck:
            try runDetached(arguments: ["usage", "--notify"])
        }
    }

    private func runAndCapture(arguments: [String]) throws -> String {
        let process = Process()
        process.executableURL = executableURL
        process.arguments = arguments

        let stdout = Pipe()
        process.standardOutput = stdout
        process.standardError = Pipe()

        try process.run()
        process.waitUntilExit()

        guard process.terminationStatus == 0 else {
            throw OctCLIServiceError.nonZeroExit(status: process.terminationStatus)
        }

        let data = stdout.fileHandleForReading.readDataToEndOfFile()
        guard let output = String(data: data, encoding: .utf8) else {
            throw OctCLIServiceError.invalidUTF8
        }
        return output
    }

    private func runDetached(arguments: [String]) throws {
        let process = Process()
        process.executableURL = executableURL
        process.arguments = arguments
        process.standardOutput = Pipe()
        process.standardError = Pipe()
        try process.run()
    }
}

enum OctCLIServiceError: Error, LocalizedError {
    case nonZeroExit(status: Int32)
    case invalidUTF8

    var errorDescription: String? {
        switch self {
        case .nonZeroExit(let status):
            return "oct exited with status \(status)"
        case .invalidUTF8:
            return "oct output was not valid UTF-8"
        }
    }
}
