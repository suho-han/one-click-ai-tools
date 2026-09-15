import Foundation

enum SettingsTab: String, CaseIterable, Identifiable {
    case configuration
    case tools

    var id: String { rawValue }

    var title: String {
        switch self {
        case .configuration:
            return "Configuration"
        case .tools:
            return "Tools"
        }
    }

    var systemImage: String {
        switch self {
        case .configuration:
            return "slider.horizontal.3"
        case .tools:
            return "terminal"
        }
    }

    var summary: String {
        switch self {
        case .configuration:
            return "Manage providers, appearance, usage display, alerts, and session refresh."
        case .tools:
            return "Run oct commands in Terminal when you need them."
        }
    }
}
