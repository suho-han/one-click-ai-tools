import Foundation

enum MenubarTitleMode: String, Codable, CaseIterable, Identifiable {
    case oct
    case compact

    var id: String { rawValue }

    var label: String {
        switch self {
        case .oct:
            return "oct"
        case .compact:
            // Shows the compact remaining-usage title, matching the popover
            // body below it.
            return "Compact %"
        }
    }
}

struct SessionRefreshIntervalOption: Equatable, Identifiable {
    let value: String
    let label: String
    let usesHour: Bool

    var id: String { value }

    static let all: [SessionRefreshIntervalOption] = [
        SessionRefreshIntervalOption(value: "1h", label: "Hourly", usesHour: false),
        SessionRefreshIntervalOption(value: "6h", label: "Every 6 hours", usesHour: false),
        SessionRefreshIntervalOption(value: "12h", label: "Every 12 hours", usesHour: false),
        SessionRefreshIntervalOption(value: "daily", label: "Daily", usesHour: true),
        SessionRefreshIntervalOption(value: "weekly", label: "Weekly", usesHour: true),
    ]

    static func option(for value: String) -> SessionRefreshIntervalOption {
        all.first { $0.value == value } ?? all[0]
    }

    static func usesHour(_ value: String) -> Bool {
        option(for: value).usesHour
    }
}


struct ConfigTool: Codable, Equatable, Identifiable {
    let name: String
    let binaryName: String
    var enabled: Bool
    /// Installed version, present only in snapshots that probed for it
    /// (`config list --json --probe-versions`, used by the settings load).
    /// nil = the installed oct is too old to report versions; "" = probed but
    /// not installed. Synthesized Codable decoding reads optionals with
    /// decodeIfPresent, so older snapshots decode unchanged.
    var version: String?

    var id: String { binaryName }

    enum CodingKeys: String, CodingKey {
        case name
        case binaryName = "binary_name"
        case enabled
        case version
    }
}

/// The platform scheduler's persisted agent-update schedule, mirrored from
/// the oct snapshot (`agent_update_schedule`). `interval` is empty when the
/// schedule is off or its interval cannot be recovered from the platform
/// (hand-edited schedules) — that state round-trips unchanged instead of
/// being rewritten to a guessed value.
struct AgentUpdateSchedule: Codable, Equatable {
    var enabled: Bool
    var interval: String
    var hour: Int

    static let off = AgentUpdateSchedule(enabled: false, interval: "", hour: 9)

    enum CodingKeys: String, CodingKey {
        case enabled
        case interval
        case hour
    }

    init(enabled: Bool, interval: String, hour: Int) {
        self.enabled = enabled
        self.interval = interval
        self.hour = hour
    }

    /// Tolerant decoding: snapshots from an older oct CLI carry no
    /// `agent_update_schedule`.
    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.init(
            enabled: try container.decodeIfPresent(Bool.self, forKey: .enabled) ?? false,
            interval: try container.decodeIfPresent(String.self, forKey: .interval) ?? "",
            hour: try container.decodeIfPresent(Int.self, forKey: .hour) ?? 9
        )
    }

    /// Sentinel picker tag for "on with an unrecoverable interval".
    static let customTag = "custom"
    /// Sentinel picker tag for "off".
    static let offTag = "off"

    var isCustom: Bool { enabled && interval.isEmpty }

    /// Picker tag for the current state: off, a concrete interval, or the
    /// custom sentinel for an enabled-but-unknown schedule.
    var pickerValue: String {
        guard enabled else { return Self.offTag }
        return interval.isEmpty ? Self.customTag : interval
    }

    /// Applies a picker tag. "custom" keeps the interval empty so an unknown
    /// schedule round-trips instead of being rewritten.
    mutating func setPickerValue(_ value: String) {
        if value == Self.offTag {
            self = .off
            return
        }
        enabled = true
        if value != Self.customTag {
            interval = value
        }
    }

    var usesHour: Bool { SessionRefreshIntervalOption.usesHour(interval) }
}

struct AlertThresholds: Codable, Equatable {
    var defaultThreshold: Double
    var fiveHours: Double
    var sevenDays: Double
    var monthly: Double

    enum CodingKeys: String, CodingKey {
        case defaultThreshold = "default"
        case fiveHours = "5h"
        case sevenDays = "7d"
        case monthly = "1m"
    }

    static let goDefaults = AlertThresholds(defaultThreshold: 80, fiveHours: 80, sevenDays: 80, monthly: 80)

    init(
        defaultThreshold: Double,
        fiveHours: Double,
        sevenDays: Double,
        monthly: Double
    ) {
        self.defaultThreshold = defaultThreshold
        self.fiveHours = fiveHours
        self.sevenDays = sevenDays
        self.monthly = monthly
    }

    /// Tolerant decoding: snapshots from an older oct CLI carry no `1m`
    /// (and legacy ones may omit other window keys), and the alerts screen
    /// must still load instead of failing with a missing-data error.
    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let defaults = AlertThresholds.goDefaults
        self.init(
            defaultThreshold: try container.decodeIfPresent(Double.self, forKey: .defaultThreshold) ?? defaults.defaultThreshold,
            fiveHours: try container.decodeIfPresent(Double.self, forKey: .fiveHours) ?? defaults.fiveHours,
            sevenDays: try container.decodeIfPresent(Double.self, forKey: .sevenDays) ?? defaults.sevenDays,
            monthly: try container.decodeIfPresent(Double.self, forKey: .monthly) ?? defaults.monthly
        )
    }
}

struct AlertSettings: Codable, Equatable {
    var enabled: Bool
    var thresholdPercent: Double
    var cooldownMinutes: Int
    var quietUntil: String
    var thresholds: AlertThresholds

    static let goDefaults = AlertSettings(
        enabled: false,
        thresholdPercent: 80,
        cooldownMinutes: 360,
        quietUntil: "",
        thresholds: .goDefaults
    )

    /// Preset quiet timer durations in hours, mirroring the Go CLI choices.
    static let quietChoices: [Int] = [1, 2, 4, 6, 12]

    /// Preset cooldown durations in minutes offered by the settings picker.
    /// Every preset satisfies the CLI payload contract (positive, ≤ 24h).
    static let cooldownChoices: [Int] = [15, 30, 60, 120, 180, 360, 720, 1440]

    /// Picker choices for cooldown: the presets plus the configured value when
    /// the CLI wrote a custom duration, so an out-of-preset selection still
    /// has a matching tag instead of rendering as a blank menu item.
    static func cooldownMenuChoices(current: Int) -> [Int] {
        guard current > 0, !cooldownChoices.contains(current) else { return cooldownChoices }
        return (cooldownChoices + [current]).sorted()
    }

    /// Human label for a cooldown duration: minutes below an hour, otherwise
    /// hours with a single decimal for fractional values ("15 min", "6 hr",
    /// "1.5 hr").
    static func cooldownLabel(for minutes: Int) -> String {
        guard minutes >= 60 else { return "\(minutes) min" }
        let hours = Double(minutes) / 60
        if hours == hours.rounded() {
            return "\(Int(hours)) hr"
        }
        return String(format: "%.1f hr", hours)
    }

    /// Maps a quiet_until timestamp to the picker bucket shown for it: 0 when
    /// the timer is off or expired, otherwise the smallest preset covering the
    /// remaining time. Purely cosmetic — the settings save round-trips
    /// quiet_until itself and only rewrites it when the picker changes.
    static func quietChoiceHours(for quietUntil: String, now: Date = Date()) -> Int {
        guard let until = parseQuietUntil(quietUntil), until > now else { return 0 }
        let remainingHours = until.timeIntervalSince(now) / 3600
        for choice in quietChoices where remainingHours <= Double(choice) {
            return choice
        }
        return quietChoices.last!
    }

    /// Builds the quiet_until value the Go config stores: an RFC3339 timestamp
    /// N hours from now, or an empty string when the timer is off.
    static func quietUntilString(armingHours hours: Int, now: Date = Date()) -> String {
        guard hours > 0 else { return "" }
        return ISO8601DateFormatter().string(from: now.addingTimeInterval(Double(hours) * 3600))
    }

    static func parseQuietUntil(_ raw: String) -> Date? {
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return nil }
        return ISO8601DateFormatter().date(from: trimmed)
    }

    enum CodingKeys: String, CodingKey {
        case enabled
        case thresholdPercent = "threshold_percent"
        case cooldownMinutes = "cooldown_minutes"
        case quietUntil = "quiet_until"
        case thresholds
    }

    init(
        enabled: Bool,
        thresholdPercent: Double,
        cooldownMinutes: Int,
        quietUntil: String,
        thresholds: AlertThresholds
    ) {
        self.enabled = enabled
        self.thresholdPercent = thresholdPercent
        self.cooldownMinutes = cooldownMinutes
        self.quietUntil = quietUntil
        self.thresholds = thresholds
    }

    /// Tolerant decoding: snapshots from an older oct CLI carry no
    /// `quiet_until` (they still had quiet_hours/timezone), and the alerts
    /// screen must still load instead of failing with a missing-data error.
    /// `critical_percent` was removed from the payload; older Go snapshots
    /// that still carry it decode fine because extra keys are ignored.
    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let defaults = AlertSettings.goDefaults
        self.init(
            enabled: try container.decodeIfPresent(Bool.self, forKey: .enabled) ?? defaults.enabled,
            thresholdPercent: try container.decodeIfPresent(Double.self, forKey: .thresholdPercent) ?? defaults.thresholdPercent,
            cooldownMinutes: try container.decodeIfPresent(Int.self, forKey: .cooldownMinutes) ?? defaults.cooldownMinutes,
            quietUntil: try container.decodeIfPresent(String.self, forKey: .quietUntil) ?? "",
            thresholds: try container.decodeIfPresent(AlertThresholds.self, forKey: .thresholds) ?? defaults.thresholds
        )
    }
}

struct ConfigurationSnapshot: Codable, Equatable {
    let configFile: String
    let menubarTitleMode: MenubarTitleMode
    let menubarRefreshInterval: String
    let sessionRefreshEnabled: Bool
    let sessionRefreshInterval: String
    let sessionRefreshHour: Int
    let agentUpdateSchedule: AgentUpdateSchedule
    /// Mutable so a background agent update can publish fresh tool versions
    /// into the snapshot (in lockstep with the draft) without a re-probe.
    var tools: [ConfigTool]
    let alert: AlertSettings

    enum CodingKeys: String, CodingKey {
        case configFile = "config_file"
        case menubarTitleMode = "menubar_title_mode"
        case menubarRefreshInterval = "menubar_refresh_interval"
        case sessionRefreshEnabled = "session_refresh_enabled"
        case sessionRefreshInterval = "session_refresh_interval"
        case sessionRefreshHour = "session_refresh_hour"
        case agentUpdateSchedule = "agent_update_schedule"
        case tools
        case alert
    }

    init(
        configFile: String,
        menubarTitleMode: MenubarTitleMode,
        menubarRefreshInterval: String = "1m",
        sessionRefreshEnabled: Bool,
        sessionRefreshInterval: String,
        sessionRefreshHour: Int,
        agentUpdateSchedule: AgentUpdateSchedule = .off,
        tools: [ConfigTool],
        alert: AlertSettings = .goDefaults
    ) {
        self.configFile = configFile
        self.menubarTitleMode = menubarTitleMode
        self.menubarRefreshInterval = menubarRefreshInterval
        self.sessionRefreshEnabled = sessionRefreshEnabled
        self.sessionRefreshInterval = sessionRefreshInterval
        self.sessionRefreshHour = sessionRefreshHour
        self.agentUpdateSchedule = agentUpdateSchedule
        self.tools = tools
        self.alert = alert
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.init(
            configFile: try container.decode(String.self, forKey: .configFile),
            menubarTitleMode: try container.decodeIfPresent(MenubarTitleMode.self, forKey: .menubarTitleMode) ?? .oct,
            menubarRefreshInterval: try container.decodeIfPresent(String.self, forKey: .menubarRefreshInterval) ?? "1m",
            sessionRefreshEnabled: try container.decode(Bool.self, forKey: .sessionRefreshEnabled),
            sessionRefreshInterval: try container.decode(String.self, forKey: .sessionRefreshInterval),
            sessionRefreshHour: try container.decode(Int.self, forKey: .sessionRefreshHour),
            agentUpdateSchedule: try container.decodeIfPresent(AgentUpdateSchedule.self, forKey: .agentUpdateSchedule) ?? .off,
            tools: try container.decode([ConfigTool].self, forKey: .tools),
            alert: try container.decodeIfPresent(AlertSettings.self, forKey: .alert) ?? .goDefaults
        )
    }

    /// Refresh cadence in seconds, mirroring the legacy Go menubar: a Go
    /// duration string ("90s", "1m30s", "1h"); 0/negative/invalid fall back
    /// to 60 seconds.
    var refreshInterval: TimeInterval {
        Self.parseGoDuration(menubarRefreshInterval) ?? 60
    }

    /// Parses Go duration syntax ("500ms", "90s", "1m30s", "2h"). Returns nil
    /// for unparseable or non-positive input.
    static func parseGoDuration(_ raw: String) -> TimeInterval? {
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return nil }

        var total: TimeInterval = 0
        var index = trimmed.startIndex

        func unitSeconds(_ unit: String) -> TimeInterval? {
            switch unit {
            case "ns": return 0.000000001
            case "us", "µs": return 0.000001
            case "ms": return 0.001
            case "s": return 1
            case "m": return 60
            case "h": return 3600
            default: return nil
            }
        }

        while index < trimmed.endIndex {
            let numberStart = index
            var sawDigit = false
            var sawDot = false
            while index < trimmed.endIndex {
                let ch = trimmed[index]
                if ch.isNumber { sawDigit = true; index = trimmed.index(after: index) }
                else if ch == "." && !sawDot { sawDot = true; index = trimmed.index(after: index) }
                else { break }
            }
            guard sawDigit, let value = Double(trimmed[numberStart..<index]) else { return nil }

            let unitStart = index
            while index < trimmed.endIndex, !trimmed[index].isNumber, trimmed[index] != "." {
                index = trimmed.index(after: index)
            }
            let unit = String(trimmed[unitStart..<index])
            guard let seconds = unitSeconds(unit) else { return nil }
            total += value * seconds
        }
        return total > 0 ? total : nil
    }
}

struct ConfigurationUpdatePayload: Codable, Equatable {
    let enabledTools: [String]
    let menubarTitleMode: MenubarTitleMode
    let sessionRefreshEnabled: Bool
    let sessionRefreshInterval: String
    let sessionRefreshHour: Int
    let agentUpdateSchedule: AgentUpdateSchedule
    let agentOrder: [String]
    let alert: AlertSettings

    enum CodingKeys: String, CodingKey {
        case enabledTools = "enabled_tools"
        case menubarTitleMode = "menubar_title_mode"
        case sessionRefreshEnabled = "session_refresh_enabled"
        case sessionRefreshInterval = "session_refresh_interval"
        case sessionRefreshHour = "session_refresh_hour"
        case agentUpdateSchedule = "agent_update_schedule"
        case agentOrder = "agent_order"
        case alert
    }
}

struct ConfigurationDraft: Equatable {
    var configFile: String
    var menubarTitleMode: MenubarTitleMode
    var sessionRefreshEnabled: Bool
    var sessionRefreshInterval: String
    var sessionRefreshHour: Int
    var agentUpdateSchedule: AgentUpdateSchedule
    var tools: [ConfigTool]
    var alert: AlertSettings

    init(snapshot: ConfigurationSnapshot) {
        configFile = snapshot.configFile
        menubarTitleMode = snapshot.menubarTitleMode
        sessionRefreshEnabled = snapshot.sessionRefreshEnabled
        sessionRefreshInterval = snapshot.sessionRefreshInterval
        sessionRefreshHour = snapshot.sessionRefreshHour
        agentUpdateSchedule = snapshot.agentUpdateSchedule
        tools = snapshot.tools
        alert = snapshot.alert
    }

    var hasEnabledTool: Bool {
        tools.contains { $0.enabled }
    }

    mutating func setTool(_ binaryName: String, enabled: Bool) {
        guard let index = tools.firstIndex(where: { $0.binaryName == binaryName }) else {
            return
        }
        tools[index].enabled = enabled
    }

    mutating func setMenubarTitleMode(_ mode: MenubarTitleMode) {
        menubarTitleMode = mode
    }

    mutating func moveTool(_ binaryName: String, by offset: Int) {
        guard let sourceIndex = tools.firstIndex(where: { $0.binaryName == binaryName }) else {
            return
        }
        let destinationIndex = sourceIndex + offset
        guard tools.indices.contains(destinationIndex) else {
            return
        }
        tools.swapAt(sourceIndex, destinationIndex)
    }


    mutating func revert(to snapshot: ConfigurationSnapshot) {
        self = ConfigurationDraft(snapshot: snapshot)
    }

    func updatePayload() -> ConfigurationUpdatePayload {
        ConfigurationUpdatePayload(
            enabledTools: tools.filter(\.enabled).map(\.binaryName),
            menubarTitleMode: menubarTitleMode,
            sessionRefreshEnabled: sessionRefreshEnabled,
            sessionRefreshInterval: sessionRefreshInterval,
            sessionRefreshHour: sessionRefreshHour,
            agentUpdateSchedule: agentUpdateSchedule,
            agentOrder: tools.map(\.binaryName),
            alert: alert
        )
    }
}
