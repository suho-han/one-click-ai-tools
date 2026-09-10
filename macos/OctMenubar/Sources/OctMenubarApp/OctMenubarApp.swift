import SwiftUI

enum MenubarPreferences {
    static let useProviderAccentColorsKey = "oct.menubar.useProviderAccentColors"
}

@main
struct OctMenubarApp: App {
    @NSApplicationDelegateAdaptor(StatusBarController.self) private var statusBarController
    @StateObject private var configurationStore = ConfigurationStore()

    var body: some Scene {
        Settings {
            SettingsView(configurationStore: configurationStore)
        }
    }
}
