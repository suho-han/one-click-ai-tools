import AppKit
import Combine
import os
import SwiftUI

@MainActor
final class StatusBarController: NSObject, NSApplicationDelegate {
    private static let logger = Logger(subsystem: "app.oct.menubar", category: "lifecycle")
    private let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let popover = NSPopover()
    // The app's single ConfigurationStore: the popover view model uses it
    // here, and OctMenubarApp hands the same instance to the Settings scene.
    let configurationStore = ConfigurationStore()
    private lazy var viewModel = UsageViewModel(service: OctCLIService(), configurationStore: configurationStore)
    private var cancellables: Set<AnyCancellable> = []
    private let releaseMonitor = ReleaseUpdateMonitor.shared
    private var updateStatusItem: UpdateStatusItemController?

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.accessory)
        configureStatusItem()
        configurePopover()
        bindSnapshot()
        suppressAutoShownSettingsWindow()
        signalReadyIfRequested()
        updateStatusItem = UpdateStatusItemController(monitor: releaseMonitor)
        updateStatusItem?.start()
        releaseMonitor.start()
    }

    /// SwiftUI auto-shows the Settings window at launch when the app has no
    /// regular window; close it so only the status item appears. SettingsLink
    /// in the popover reopens it on demand. The window can materialize only
    /// after several run-loop ticks on slow launches, so the close attempts
    /// run for up to five seconds and log if they never succeeded.
    private func suppressAutoShownSettingsWindow() {
        // The window identifier's casing differs across macOS releases
        // ("settings" vs "Settings"); match both.
        let settingsWindowIDs: Set<String> = [
            "com_apple_SwiftUI_settings_window",
            "com_apple_SwiftUI_Settings_window",
        ]
        func closeAttempt(remaining: Int) {
            if closeSettingsWindow(identifiers: settingsWindowIDs) {
                return
            }
            if remaining <= 0 {
                Self.logger.error("Settings window never appeared to be closable; an empty settings window may be on screen")
                return
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                closeAttempt(remaining: remaining - 1)
            }
        }
        closeAttempt(remaining: 50)
    }

    private func closeSettingsWindow(identifiers: Set<String>) -> Bool {
        var closed = false
        for window in NSApp.windows where identifiers.contains(window.identifier?.rawValue ?? "") {
            window.close()
            closed = true
        }
        return closed
    }

    private func configureStatusItem() {
        guard let button = statusItem.button else { return }
        button.title = UsageSnapshot.placeholder.statusItemTitle
        button.setAccessibilityLabel(UsageSnapshot.placeholder.statusItemAccessibilityLabel)
        button.target = self
        button.action = #selector(togglePopover(_:))
    }

    private func configurePopover() {
        popover.behavior = .transient
        popover.animates = true
        popover.contentSize = PopoverView.preferredSize(for: UsageSnapshot.placeholder.providers.count)
        popover.contentViewController = NSHostingController(rootView: PopoverView(viewModel: viewModel, releaseMonitor: releaseMonitor))
    }

    private func bindSnapshot() {
        viewModel.$snapshot
            .receive(on: RunLoop.main)
            .sink { [weak self] snapshot in
                self?.statusItem.button?.title = snapshot.statusItemTitle
                self?.statusItem.button?.setAccessibilityLabel(snapshot.statusItemAccessibilityLabel)
                self?.popover.contentSize = PopoverView.preferredSize(for: snapshot.providers.count)
            }
            .store(in: &cancellables)
    }

    private func signalReadyIfRequested() {
        guard let path = ProcessInfo.processInfo.environment["OCT_MENUBAR_READY_FILE"],
              !path.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        else {
            return
        }
        do {
            try "ready\n".write(toFile: path, atomically: true, encoding: .utf8)
        } catch {
            // A silent failure would stall the parent `oct menubar` wait
            // until its own timeout with no diagnostic anywhere.
            FileHandle.standardError.write(
                Data("oct-menubar: failed to write ready file \(path): \(error)\n".utf8)
            )
        }
    }

    @objc
    private func togglePopover(_ sender: AnyObject?) {
        guard let button = statusItem.button else { return }
        if popover.isShown {
            popover.performClose(sender)
            return
        }
        // Reload-on-open policy: pick up external (CLI) config changes.
        Task { await self.configurationStore.reload() }
        popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
        NSApp.activate(ignoringOtherApps: true)
    }
}
