import Foundation
import SwiftUI

struct UsageSnapshot: Equatable {
    var summaryLine: String
    var lastRefreshLabel: String
    var nextRefreshLabel: String
    var providers: [ProviderCard]

    static let placeholder = UsageSnapshot(
        summaryLine: "Loading usage…",
        lastRefreshLabel: "-",
        nextRefreshLabel: "pending",
        providers: [
            ProviderCard(name: "codex", status: "ok", metricsLine: "5h - · 7d -", message: "Waiting for first refresh"),
            ProviderCard(name: "claude-code", status: "ok", metricsLine: "5h - · 7d -", message: nil),
        ]
    )

    static func error(message: String) -> UsageSnapshot {
        UsageSnapshot(
            summaryLine: "Refresh failed",
            lastRefreshLabel: "-",
            nextRefreshLabel: "pending",
            providers: [
                ProviderCard(name: "error", status: "error", metricsLine: "usage unavailable", message: message),
            ]
        )
    }
}

struct ProviderCard: Equatable, Identifiable {
    let id: String
    let name: String
    let status: String
    let metricsLine: String
    let message: String?

    init(name: String, status: String, metricsLine: String, message: String?) {
        self.id = name
        self.name = name
        self.status = status
        self.metricsLine = metricsLine
        self.message = message
    }

    var statusColor: Color {
        switch status {
        case "ok":
            .green
        case "warn":
            .orange
        case "error":
            .red
        default:
            .secondary
        }
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
    static func from(response: UsageResponse, refreshDate: Date, refreshInterval: TimeInterval) -> UsageSnapshot {
        let formatter = DateFormatter()
        formatter.dateFormat = "HH:mm:ss"

        return UsageSnapshot(
            summaryLine: "\(response.summary.total) providers · \(response.summary.ok) ok · \(response.summary.warn) warn · \(response.summary.error) error",
            lastRefreshLabel: formatter.string(from: refreshDate),
            nextRefreshLabel: formatter.string(from: refreshDate.addingTimeInterval(refreshInterval)),
            providers: response.results.map { result in
                let fiveHour = result.buckets?["5h"] ?? "-"
                let sevenDay = result.buckets?["7d"] ?? "-"
                return ProviderCard(
                    name: result.provider,
                    status: result.status,
                    metricsLine: "5h \(fiveHour) · 7d \(sevenDay)",
                    message: result.message
                )
            }
        )
    }
}
