import AppKit
import Combine
import SwiftUI

@MainActor
final class StatusBarController: NSObject, NSApplicationDelegate {
    private let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let popover = NSPopover()
    private let configurationStore = ConfigurationStore()
    private lazy var viewModel = UsageViewModel(service: OctCLIService(), configurationStore: configurationStore)
    private var cancellables: Set<AnyCancellable> = []

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.accessory)
        configureStatusItem()
        configurePopover()
        bindSnapshot()
        signalReadyIfRequested()
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
        popover.contentViewController = NSHostingController(rootView: PopoverView(viewModel: viewModel))
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
