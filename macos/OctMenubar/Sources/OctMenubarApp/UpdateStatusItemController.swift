import AppKit
import Combine

/// Shows a compact update status item while a newer GitHub release exists.
/// Confirmation and execution are shared with the popover's update button.
@MainActor
final class UpdateStatusItemController: NSObject {
    private var updateItem: NSStatusItem?
    private var pendingUpdate: ReleaseUpdate?
    private let monitor: ReleaseUpdateMonitor
    private var cancellables: Set<AnyCancellable> = []

    init(monitor: ReleaseUpdateMonitor) {
        self.monitor = monitor
        super.init()
    }

    func start() {
        monitor.$availableUpdate
            .receive(on: RunLoop.main)
            .sink { [weak self] update in
                self?.refresh(update: update)
            }
            .store(in: &cancellables)
        monitor.installer.$isUpdating
            .receive(on: RunLoop.main)
            .sink { [weak self] _ in
                guard let self else { return }
                self.refresh(update: self.pendingUpdate)
            }
            .store(in: &cancellables)
    }

    private func refresh(update: ReleaseUpdate?) {
        pendingUpdate = update
        if update != nil {
            if updateItem == nil {
                updateItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
            }
            if let button = updateItem?.button {
                button.title = monitor.installer.isUpdating ? "⬇️ updating…" : "⬇️ update"
                button.toolTip = monitor.installer.isUpdating ? "oct update is running" : "A newer oct release is available — click to confirm oct update"
                button.setAccessibilityLabel(monitor.installer.isUpdating ? "oct update in progress" : "Update available; confirm oct update")
                button.isEnabled = !monitor.installer.isUpdating
                button.target = self
                button.action = #selector(openUpdateButton(_:))
            }
        } else if let item = updateItem {
            NSStatusBar.system.removeStatusItem(item)
            updateItem = nil
        }
    }

    @objc
    private func openUpdateButton(_ sender: AnyObject?) {
        guard let update = pendingUpdate else { return }
        monitor.installer.requestUpdate(to: update)
    }
}
