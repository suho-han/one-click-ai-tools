import Foundation

enum SettingsTab: String, CaseIterable, Identifiable {
    case general
    case configuration
    case tools

    var id: String { rawValue }

    var title: String {
        switch self {
        case .general:
            return "General"
        case .configuration:
            return "Configuration"
        case .tools:
            return "Tools"
        }
    }

    var systemImage: String {
        switch self {
        case .general:
            return "paintbrush"
        case .configuration:
            return "slider.horizontal.3"
        case .tools:
            return "terminal"
        }
    }

    var summary: String {
        switch self {
        case .general:
            return "Appearance preferences for the menubar utility."
        case .configuration:
            return "Manage providers, usage display, and session refresh."
        case .tools:
            return "Run oct commands in Terminal when you need them."
        }
    }
}
