import AppKit
import Combine

/// Shows a compact "⬇️ update" status item only while a newer GitHub release
/// is available; clicking it opens the release download page (a one-purpose
/// button gets a direct action, not a menu — the monitor already re-checks
/// every five minutes). The version text itself lives in the popover header:
/// as a menu bar item it was invisible on crowded bars, hidden inside the «
/// overflow. The monitor is owned by StatusBarController and shared with the
/// popover header.
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
    }

    private func refresh(update: ReleaseUpdate?) {
        pendingUpdate = update
        if update != nil {
            if updateItem == nil {
                let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
                if let button = item.button {
                    button.title = "⬇️ update"
                    button.toolTip = "A newer oct release is available — click to open the download page"
                    button.setAccessibilityLabel("Update available; open the release download page")
                    button.target = self
                    button.action = #selector(openUpdateButton(_:))
                }
                updateItem = item
            }
        } else if let item = updateItem {
            NSStatusBar.system.removeStatusItem(item)
            updateItem = nil
        }
    }

    @objc
    private func openUpdateButton(_ sender: AnyObject?) {
        guard let update = pendingUpdate else { return }
        NSWorkspace.shared.open(update.htmlURL)
    }
}
