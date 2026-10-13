import SwiftUI

enum MenubarPreferences {
    static let useProviderAccentColorsKey = "oct.menubar.useProviderAccentColors"
    static let sharedAlertThresholdKey = "oct.menubar.sharedAlertThreshold"
    static let automaticallyChecksForUpdatesKey = "oct.menubar.automaticallyChecksForUpdates"
}

@main
struct OctMenubarApp: App {
    @NSApplicationDelegateAdaptor(StatusBarController.self) private var statusBarController

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
            // The status bar controller owns the app-wide ConfigurationStore;
            // the Settings scene observes that same instance so a save from
            // either side reaches the other.
            SettingsView(configurationStore: statusBarController.configurationStore)
        }
    }
}
