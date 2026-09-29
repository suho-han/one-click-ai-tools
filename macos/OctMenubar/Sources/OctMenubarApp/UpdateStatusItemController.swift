import AppKit
import Combine

/// Owns the auxiliary menubar items for update notifications: an
/// always-visible "One Click AI Tools v<n>" item whose menu offers a manual
/// check, and — once a newer release is detected — an adjacent "⬇️ update"
/// button that opens the release download page. Instantiated after the
/// usage status item, so the button lands next to (left of) the version
/// text.
@MainActor
final class UpdateStatusItemController: NSObject {
    private let versionItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private var updateItem: NSStatusItem?
    private var pendingUpdate: ReleaseUpdate?
    private let monitor: ReleaseUpdateMonitor
    private var cancellables: Set<AnyCancellable> = []

    init(monitor: ReleaseUpdateMonitor = ReleaseUpdateMonitor()) {
        self.monitor = monitor
        super.init()
    }

    func start() {
        configureVersionItem()
        monitor.start()
        monitor.$availableUpdate
            .receive(on: RunLoop.main)
            .sink { [weak self] update in
                self?.refresh(update: update)
            }
            .store(in: &cancellables)
    }

    private func configureVersionItem() {
        guard let button = versionItem.button else { return }
        button.attributedTitle = Self.versionTitle()
        button.setAccessibilityLabel("One Click AI Tools, version \(MenubarBuildVersion.version)")
        versionItem.menu = makeMenu(update: nil)
    }

    /// "One Click AI Tools" at menu-bar size with the version beside it in
    /// smaller, dimmer type.
    private static func versionTitle() -> NSAttributedString {
        let raw = MenubarBuildVersion.version
        let versionText = raw == "dev" || raw.hasPrefix("v") ? raw : "v\(raw)"
        let title = NSMutableAttributedString(
            string: "One Click AI Tools ",
            attributes: [
                .font: NSFont.systemFont(ofSize: 13),
                .foregroundColor: NSColor.labelColor,
            ]
        )
        title.append(NSAttributedString(
            string: versionText,
            attributes: [
                .font: NSFont.systemFont(ofSize: 10),
                .foregroundColor: NSColor.secondaryLabelColor,
            ]
        ))
        return title
    }

    private func makeMenu(update: ReleaseUpdate?) -> NSMenu {
        let menu = NSMenu()
        if let update {
            let item = NSMenuItem(
                title: "⬇️ Update to \(update.tag)…",
                action: #selector(openUpdatePage(_:)),
                keyEquivalent: ""
            )
            item.target = self
            menu.addItem(item)
            menu.addItem(.separator())
        }
        let current = NSMenuItem(
            title: "Current version: \(MenubarBuildVersion.version)",
            action: nil,
            keyEquivalent: ""
        )
        current.isEnabled = false
        menu.addItem(current)
        let check = NSMenuItem(
            title: "Check for updates now",
            action: #selector(checkNowManually(_:)),
            keyEquivalent: ""
        )
        check.target = self
        menu.addItem(check)
        return menu
    }

    private func refresh(update: ReleaseUpdate?) {
        pendingUpdate = update
        if update != nil {
            if updateItem == nil {
                let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
                item.button?.title = "⬇️ update"
                item.button?.toolTip = "A newer oct release is available — click to open the download page"
                item.button?.setAccessibilityLabel("Update available; open the release download page")
                item.button?.target = self
                item.button?.action = #selector(openUpdateButton(_:))
                updateItem = item
            }
        } else if let item = updateItem {
            NSStatusBar.system.removeStatusItem(item)
            updateItem = nil
        }
        versionItem.menu = makeMenu(update: update)
    }

    @objc
    private func openUpdateButton(_ sender: AnyObject?) {
        openReleasePage()
    }

    @objc
    private func openUpdatePage(_ sender: AnyObject?) {
        openReleasePage()
    }

    @objc
    private func checkNowManually(_ sender: AnyObject?) {
        Task { await monitor.checkNow() }
    }

    private func openReleasePage() {
        guard let update = pendingUpdate else { return }
        NSWorkspace.shared.open(update.htmlURL)
    }
}
