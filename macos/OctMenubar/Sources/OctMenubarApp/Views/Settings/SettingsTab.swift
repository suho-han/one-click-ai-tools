import Foundation

enum SettingsTab: String, CaseIterable, Identifiable {
    case providers
    case display
    case alerts
    case automation
    case about

    var id: String { rawValue }

    var title: String {
        switch self {
        case .providers: "Providers"
        case .display: "Display"
        case .alerts: "Alerts"
        case .automation: "Automation"
        case .about: "About"
        }
    }

    var systemImage: String {
        switch self {
        case .providers: "person.2"
        case .display: "paintpalette"
        case .alerts: "bell.badge"
        case .automation: "arrow.clockwise.circle"
        case .about: "info.circle"
        }
    }

    var summary: String {
        switch self {
        case .providers: "Choose providers, check versions, and schedule updates."
        case .display: "Customize the menu bar and provider appearance."
        case .alerts: "Set usage alert thresholds and quiet periods."
        case .automation: "Schedule session refresh and launch at login."
        case .about: "App details, updates, and project links."
        }
    }
}
