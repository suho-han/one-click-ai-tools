import Foundation
import SwiftUI

struct UsageSnapshot: Equatable {
    let statusItemTitle: String
    let title: String
    let summaryLine: String
    let lastRefreshLabel: String
    let nextRefreshLabel: String
    let autoRefreshLabel: String
    let providers: [ProviderCard]
    let note: String?

    static let placeholder = UsageSnapshot(
        statusItemTitle: "oct …",
        title: "Usage Overview",
        summaryLine: "Loading usage…",
        lastRefreshLabel: "-",
        nextRefreshLabel: "pending",
        autoRefreshLabel: "Auto refresh: every 1m",
        providers: [
            ProviderCard(name: "codex", status: .loading, metrics: [.init(label: "5h", value: "-"), .init(label: "7d", value: "-")], message: "Waiting for first refresh"),
            ProviderCard(name: "claude-code", status: .loading, metrics: [.init(label: "5h", value: "-"), .init(label: "7d", value: "-")], message: "Refresh timer starts after launch"),
        ],
        note: "Data will be loaded from oct usage --json after launch."
    )

    static func error(message: String, refreshInterval: TimeInterval = 60) -> UsageSnapshot {
        UsageSnapshot(
            statusItemTitle: "oct !!",
            title: "Usage Overview",
            summaryLine: "Refresh failed",
            lastRefreshLabel: "-",
            nextRefreshLabel: "pending",
            autoRefreshLabel: "Auto refresh: every \(DurationFormatter.label(for: refreshInterval))",
            providers: [
                ProviderCard(name: "oct", status: .error, metrics: [.init(label: "5h", value: "unavailable"), .init(label: "7d", value: "unavailable")], message: message),
            ],
            note: "Check oct path or run Refresh now after fixing the CLI location."
        )
    }
}

struct ProviderCard: Equatable, Identifiable {
    let id: String
    let name: String
    let status: ProviderStatus
    let metrics: [UsageMetric]
    let message: String?

    init(name: String, status: ProviderStatus, metrics: [UsageMetric], message: String?) {
        self.id = name
        self.name = name
        self.status = status
        self.metrics = metrics
        self.message = message
    }

    var metricsLine: String {
        metrics.map { "\($0.label) \($0.value)" }.joined(separator: " · ")
    }
}

struct UsageMetric: Equatable {
    let label: String
    let value: String
}

enum ProviderStatus: String, Decodable, Equatable {
    case ok
    case warn
    case error
    case loading

    var badgeLabel: String {
        rawValue.uppercased()
    }

    var tint: Color {
        switch self {
        case .ok:
            return Color(nsColor: .systemGreen)
        case .warn:
            return Color(nsColor: .systemOrange)
        case .error:
            return Color(nsColor: .systemRed)
        case .loading:
            return Color(nsColor: .systemGray)
        }
    }

    var softBackground: Color {
        tint.opacity(0.14)
    }
}

struct UsageResponse: Decodable, Equatable {
    struct Summary: Decodable, Equatable {
        let total: Int
        let ok: Int
        let warn: Int
        let error: Int
    }

    struct Result: Decodable, Equatable {
        let provider: String
        let status: String
        let used: String
        let unit: String
        let buckets: [String: String]?
        let message: String?
    }

    let summary: Summary
    let results: [Result]
}

extension UsageSnapshot {
    static func from(
        response: UsageResponse,
        refreshDate: Date,
        refreshInterval: TimeInterval,
        timeZone: TimeZone = .autoupdatingCurrent
    ) -> UsageSnapshot {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = "HH:mm:ss"

        let status = aggregateStatus(summary: response.summary)
        return UsageSnapshot(
            statusItemTitle: statusItemTitle(for: status),
            title: "Usage Overview",
            summaryLine: "\(response.summary.total) providers · \(response.summary.ok) ok · \(response.summary.warn) warn · \(response.summary.error) error",
            lastRefreshLabel: formatter.string(from: refreshDate),
            nextRefreshLabel: formatter.string(from: refreshDate.addingTimeInterval(refreshInterval)),
            autoRefreshLabel: "Auto refresh: every \(DurationFormatter.label(for: refreshInterval))",
            providers: response.results.map { result in
                ProviderCard(
                    name: result.provider,
                    status: effectiveStatus(for: result),
                    metrics: [
                        UsageMetric(label: "5h", value: result.buckets?["5h"] ?? "-"),
                        UsageMetric(label: "7d", value: result.buckets?["7d"] ?? "-"),
                    ],
                    message: result.message
                )
            },
            note: status == .error ? "One or more providers need attention." : "Data comes from oct usage --json without submitting prompts."
        )
    }

    private static func aggregateStatus(summary: UsageResponse.Summary) -> ProviderStatus {
        if summary.error > 0 {
            return .error
        }
        if summary.warn > 0 {
            return .warn
        }
        return .ok
    }

    private static func statusItemTitle(for status: ProviderStatus) -> String {
        switch status {
        case .error:
            return "oct !!"
        case .warn:
            return "oct !"
        case .loading:
            return "oct …"
        case .ok:
            return "oct"
        }
    }

    private static func classifyStatus(_ raw: String) -> String {
        switch raw.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
        case "", "ok":
            return "ok"
        case "warn":
            return "warn"
        case "loading":
            return "loading"
        default:
            return "error"
        }
    }

    private static func effectiveStatus(for result: UsageResponse.Result) -> ProviderStatus {
        let normalized = classifyStatus(result.status)
        guard normalized == "ok" else {
            return ProviderStatus(rawValue: normalized) ?? .error
        }

        let used = result.used.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        if used.isEmpty || used == "n/a" {
            return .warn
        }

        let message = result.message?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() ?? ""
        if hasNoDataSignal(message) || hasPartialSignal(message) {
            return .warn
        }

        return .ok
    }

    private static func hasNoDataSignal(_ message: String) -> Bool {
        message.hasPrefix("no data:") ||
        message.hasPrefix("no ") ||
        message.contains("not found") ||
        message.contains("no configured") ||
        message.contains("no usage metrics")
    }

    private static func hasPartialSignal(_ message: String) -> Bool {
        message.hasPrefix("partial:") || message.contains("partial data")
    }
}

enum DurationFormatter {
    static func label(for interval: TimeInterval) -> String {
        let totalSeconds = max(Int(interval.rounded()), 1)
        let minutes = totalSeconds / 60
        let seconds = totalSeconds % 60
        if minutes > 0 && seconds > 0 {
            return "\(minutes)m\(seconds)s"
        }
        if minutes > 0 {
            return "\(minutes)m"
        }
        return "\(seconds)s"
    }
}
