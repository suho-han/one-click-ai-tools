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

    init(service: OctCLIService = OctCLIService()) {
        self.service = service
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
            let result = NSAlert()
            do {
                let output = try await service.runSelfUpdate()
                result.messageText = "oct update finished"
                let detail = output.trimmingCharacters(in: .whitespacesAndNewlines)
                result.informativeText = "\(detail.isEmpty ? "oct update completed." : detail)\nRestart the menu bar app to use the updated helper."
                result.alertStyle = .informational
            } catch {
                result.messageText = "oct update failed"
                result.informativeText = error.localizedDescription
                result.alertStyle = .warning
            }
            isUpdating = false
            result.runModal()
        }
    }
}
