import SwiftUI

@main
struct OctMenubarApp: App {
    @NSApplicationDelegateAdaptor(StatusBarController.self) private var statusBarController

    var body: some Scene {
        Settings {
            EmptyView()
        }
    }
}
