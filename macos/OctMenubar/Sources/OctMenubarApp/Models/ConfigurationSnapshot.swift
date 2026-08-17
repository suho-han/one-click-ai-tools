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

enum MenubarTitleMode: String, Codable, CaseIterable, Identifiable {
    case oct
    case compact

    var id: String { rawValue }

    var label: String {
        switch self {
        case .oct:
            return "oct"
        case .compact:
            return "Remaining %"
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
    let menubarTitleMode: MenubarTitleMode
    let sessionRefreshEnabled: Bool
    let sessionRefreshInterval: String
    let sessionRefreshHour: Int
    let tools: [ConfigTool]

    enum CodingKeys: String, CodingKey {
        case configFile = "config_file"
        case usageDisplayMode = "usage_display_mode"
        case menubarTitleMode = "menubar_title_mode"
        case sessionRefreshEnabled = "session_refresh_enabled"
        case sessionRefreshInterval = "session_refresh_interval"
        case sessionRefreshHour = "session_refresh_hour"
        case tools
    }

    init(
        configFile: String,
        usageDisplayMode: UsageDisplayMode,
        menubarTitleMode: MenubarTitleMode,
        sessionRefreshEnabled: Bool,
        sessionRefreshInterval: String,
        sessionRefreshHour: Int,
        tools: [ConfigTool]
    ) {
        self.configFile = configFile
        self.usageDisplayMode = usageDisplayMode
        self.menubarTitleMode = menubarTitleMode
        self.sessionRefreshEnabled = sessionRefreshEnabled
        self.sessionRefreshInterval = sessionRefreshInterval
        self.sessionRefreshHour = sessionRefreshHour
        self.tools = tools
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.init(
            configFile: try container.decode(String.self, forKey: .configFile),
            usageDisplayMode: try container.decode(UsageDisplayMode.self, forKey: .usageDisplayMode),
            menubarTitleMode: try container.decodeIfPresent(MenubarTitleMode.self, forKey: .menubarTitleMode) ?? .oct,
            sessionRefreshEnabled: try container.decode(Bool.self, forKey: .sessionRefreshEnabled),
            sessionRefreshInterval: try container.decode(String.self, forKey: .sessionRefreshInterval),
            sessionRefreshHour: try container.decode(Int.self, forKey: .sessionRefreshHour),
            tools: try container.decode([ConfigTool].self, forKey: .tools)
        )
    }
}

struct ConfigurationUpdatePayload: Codable, Equatable {
    let enabledTools: [String]
    let usageDisplayMode: UsageDisplayMode
    let menubarTitleMode: MenubarTitleMode
    let sessionRefreshEnabled: Bool
    let sessionRefreshInterval: String
    let sessionRefreshHour: Int
    let agentOrder: [String]

    enum CodingKeys: String, CodingKey {
        case enabledTools = "enabled_tools"
        case usageDisplayMode = "usage_display_mode"
        case menubarTitleMode = "menubar_title_mode"
        case sessionRefreshEnabled = "session_refresh_enabled"
        case sessionRefreshInterval = "session_refresh_interval"
        case sessionRefreshHour = "session_refresh_hour"
        case agentOrder = "agent_order"
    }
}

struct ConfigurationDraft: Equatable {
    var configFile: String
    var usageDisplayMode: UsageDisplayMode
    var menubarTitleMode: MenubarTitleMode
    var sessionRefreshEnabled: Bool
    var sessionRefreshInterval: String
    var sessionRefreshHour: Int
    var tools: [ConfigTool]

    init(snapshot: ConfigurationSnapshot) {
        configFile = snapshot.configFile
        usageDisplayMode = snapshot.usageDisplayMode
        menubarTitleMode = snapshot.menubarTitleMode
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
            usageDisplayMode: usageDisplayMode,
            menubarTitleMode: menubarTitleMode,
            sessionRefreshEnabled: sessionRefreshEnabled,
            sessionRefreshInterval: sessionRefreshInterval,
            sessionRefreshHour: sessionRefreshHour,
            agentOrder: tools.map(\.binaryName)
        )
    }
}
