import Foundation

enum UsageDisplayMode: String, Codable, CaseIterable, Identifiable {
    case remaining
    case used

    var id: String { rawValue }

    var label: String {
        switch self {
        case .remaining:
            return "Remaining"
        case .used:
            return "Used"
        }
    }
}

struct ConfigTool: Codable, Equatable, Identifiable {
    let name: String
    let binaryName: String
    var enabled: Bool

    var id: String { binaryName }

    enum CodingKeys: String, CodingKey {
        case name
        case binaryName = "binary_name"
        case enabled
    }
}

struct ConfigurationSnapshot: Codable, Equatable {
    let configFile: String
    let usageDisplayMode: UsageDisplayMode
    let sessionRefreshEnabled: Bool
    let sessionRefreshInterval: String
    let sessionRefreshHour: Int
    let tools: [ConfigTool]

    enum CodingKeys: String, CodingKey {
        case configFile = "config_file"
        case usageDisplayMode = "usage_display_mode"
        case sessionRefreshEnabled = "session_refresh_enabled"
        case sessionRefreshInterval = "session_refresh_interval"
        case sessionRefreshHour = "session_refresh_hour"
        case tools
    }
}

struct ConfigurationUpdatePayload: Codable, Equatable {
    let enabledTools: [String]
    let usageDisplayMode: UsageDisplayMode
    let sessionRefreshEnabled: Bool
    let sessionRefreshInterval: String
    let sessionRefreshHour: Int

    enum CodingKeys: String, CodingKey {
        case enabledTools = "enabled_tools"
        case usageDisplayMode = "usage_display_mode"
        case sessionRefreshEnabled = "session_refresh_enabled"
        case sessionRefreshInterval = "session_refresh_interval"
        case sessionRefreshHour = "session_refresh_hour"
    }
}

struct ConfigurationDraft: Equatable {
    var configFile: String
    var usageDisplayMode: UsageDisplayMode
    var sessionRefreshEnabled: Bool
    var sessionRefreshInterval: String
    var sessionRefreshHour: Int
    var tools: [ConfigTool]

    init(snapshot: ConfigurationSnapshot) {
        configFile = snapshot.configFile
        usageDisplayMode = snapshot.usageDisplayMode
        sessionRefreshEnabled = snapshot.sessionRefreshEnabled
        sessionRefreshInterval = snapshot.sessionRefreshInterval
        sessionRefreshHour = snapshot.sessionRefreshHour
        tools = snapshot.tools
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

    mutating func revert(to snapshot: ConfigurationSnapshot) {
        self = ConfigurationDraft(snapshot: snapshot)
    }

    func updatePayload() -> ConfigurationUpdatePayload {
        ConfigurationUpdatePayload(
            enabledTools: tools.filter(\.enabled).map(\.binaryName),
            usageDisplayMode: usageDisplayMode,
            sessionRefreshEnabled: sessionRefreshEnabled,
            sessionRefreshInterval: sessionRefreshInterval,
            sessionRefreshHour: sessionRefreshHour
        )
    }
}
