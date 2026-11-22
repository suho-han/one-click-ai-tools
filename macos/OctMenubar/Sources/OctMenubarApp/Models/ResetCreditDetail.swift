import Foundation

struct ResetCreditDetail: Decodable, Equatable {
    let scope: String?
    let expiresAt: String?
    let count: Int

    enum CodingKeys: String, CodingKey {
        case scope, count
        case expiresAt = "expires_at"
    }

    static func codexScope(for title: String?) -> String? {
        let title = title?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() ?? ""
        let prefix = "full reset ("
        guard title.hasPrefix(prefix), title.hasSuffix(")") else { return nil }
        let rawPeriods = title.dropFirst(prefix.count).dropLast().split(separator: "+")
        var periods: [String] = []
        for rawPeriod in rawPeriods {
            switch rawPeriod.trimmingCharacters(in: .whitespaces) {
            case "5h", "5 hr", "5 hours": periods.append("5h")
            case "weekly": periods.append("Weekly")
            case "monthly": periods.append("Monthly")
            default: return nil
            }
        }
        return periods.isEmpty ? nil : "All models · \(periods.joined(separator: " + "))"
    }

    static func hoverText(for details: [Self], now: Date) -> String? {
        var lines: [String] = []
        for credit in details where credit.count > 0 {
            let expiry: String
            if let date = ResetCountdown.parseResetTime(credit.expiresAt) {
                guard let remaining = ResetCountdown.text(for: date.timeIntervalSince(now)) else { continue }
                expiry = "\(date.formatted(date: .abbreviated, time: .shortened)) (\(remaining))"
            } else {
                expiry = credit.expiresAt == nil || credit.expiresAt == "" ? "No expiry reported" : "Expiry unavailable"
            }
            let scope = credit.scope ?? "Scope unavailable"
            for _ in 0..<credit.count {
                lines.append("#\(lines.count + 1) - \(scope)\n    \(expiry)")
            }
        }
        return lines.isEmpty ? nil : lines.joined(separator: "\n")
    }
}
