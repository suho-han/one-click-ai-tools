import AppKit
import Combine
import Foundation

/// One update action shared by the status item and the popover. The CLI uses
/// the same installed oct path as the rest of the helper, not a shell PATH
/// lookup (which can select the other stable/beta track).
@MainActor
final class ReleaseUpdateInstaller: ObservableObject {
    @Published private(set) var isUpdating = false
    private let service: OctCLIService

    /// Presents the outcome dialog and returns the pressed button. Swappable
    /// in tests, where a modal would block the run.
    var presentResult: (NSAlert) -> NSApplication.ModalResponse = { $0.runModal() }

    /// Hands this helper over to the freshly installed one: the replacement is
    /// queued in a detached shell before the app quits, so the menubar never
    /// stays down. Swappable in tests, where quitting would end the run.
    var restartHelper: @MainActor () throws -> Void

    init(service: OctCLIService = OctCLIService()) {
        self.service = service
        self.restartHelper = {
            try service.restartHelperDetached()
            NSApp.terminate(nil)
        }
    }

    func requestUpdate(to update: ReleaseUpdate) {
        guard !isUpdating else { return }
        let alert = NSAlert()
        alert.messageText = "Update oct to \(update.tag)?"
        alert.informativeText = "Run oct update for this installation now? No update will run unless you choose Yes."
        alert.alertStyle = .informational
        alert.addButton(withTitle: "Yes")
        alert.addButton(withTitle: "No")
        NSApp.activate(ignoringOtherApps: true)
        guard alert.runModal() == .alertFirstButtonReturn else { return }

        // Set synchronously before scheduling the task so a second click
        // cannot start another updater while the first is being launched.
        isUpdating = true
        Task {
            await performUpdate()
        }
    }

    /// Runs the update without the consent alert for users who opted in
    /// (first-install question or Settings toggle). The outcome dialog still
    /// shows, so an automatic update is never silent.
    func requestAutomaticUpdate() {
        guard !isUpdating else { return }
        isUpdating = true
        Task {
            await performUpdate()
        }
    }

    /// Runs `oct update` and presents the outcome. A successful update offers
    /// to restart right away so the new helper takes over without a trip to
    /// the menu; the failure dialog has no such button because there is
    /// nothing new to restart into. Split from requestUpdate so tests can
    /// drive the CLI flow without the confirmation dialog.
    func performUpdate() async {
        let result = NSAlert()
        var offersRestart = false
        do {
            let output = try await service.runSelfUpdate()
            result.messageText = "oct update finished"
            let detail = output.trimmingCharacters(in: .whitespacesAndNewlines)
            result.informativeText = "\(detail.isEmpty ? "oct update completed." : detail)\nRestart the menu bar app to use the updated helper."
            result.alertStyle = .informational
            result.addButton(withTitle: "Restart")
            result.addButton(withTitle: "Later")
            offersRestart = true
        } catch {
            result.messageText = "oct update failed"
            result.informativeText = error.localizedDescription
            result.alertStyle = .warning
        }
        isUpdating = false
        let response = presentResult(result)
        // The failure alert's lone OK button also reports first-button, so
        // the response alone cannot authorize a restart.
        guard offersRestart, response == .alertFirstButtonReturn else { return }
        do {
            try restartHelper()
        } catch {
            let failure = NSAlert()
            failure.messageText = "Restart failed"
            failure.informativeText = "\(error.localizedDescription)\nQuit and reopen the menu bar app to use the updated helper."
            failure.alertStyle = .warning
            _ = presentResult(failure)
        }
    }
}
