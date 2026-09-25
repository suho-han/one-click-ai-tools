import SwiftUI

enum MenubarPreferences {
    static let useProviderAccentColorsKey = "oct.menubar.useProviderAccentColors"
}

@main
struct OctMenubarApp: App {
    @NSApplicationDelegateAdaptor(StatusBarController.self) private var statusBarController
    @StateObject private var configurationStore = ConfigurationStore()

    init() {
        // `--version` prints the stamped build version and exits before the
        // status item starts; `oct menubar doctor` uses it to surface a
        // helper that lags behind the oct binary.
        if CommandLine.arguments.contains("--version") {
            print("OctMenubarApp \(MenubarBuildVersion.version)")
            exit(0)
        }
    }

    var body: some Scene {
        Settings {
            SettingsView(configurationStore: configurationStore)
        }
    }
}
