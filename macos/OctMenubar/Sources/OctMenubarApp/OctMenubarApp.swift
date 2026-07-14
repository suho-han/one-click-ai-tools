import SwiftUI

enum MenubarPreferences {
    static let useProviderAccentColorsKey = "oct.menubar.useProviderAccentColors"
}

@main
struct OctMenubarApp: App {
    @NSApplicationDelegateAdaptor(StatusBarController.self) private var statusBarController

    var body: some Scene {
        Settings {
            SettingsView()
        }
    }
}
