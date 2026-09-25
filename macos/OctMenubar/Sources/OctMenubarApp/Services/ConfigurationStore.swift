import Foundation
import Combine

/// Single owner of the oct configuration cache. Both the menubar view model
/// and the settings window observe this store, so a save from one side
/// reaches the other (including the refresh timer); external CLI changes are
/// picked up by the explicit reload-on-open policy.
@MainActor
final class ConfigurationStore: ObservableObject {
    @Published private(set) var snapshot: ConfigurationSnapshot?
    @Published var draft: ConfigurationDraft?
    @Published var isLoading = false
    @Published var isSaving = false
    @Published var feedback: SettingsFeedback?

    private let service: OctCLIService
    /// Guards against a late-arriving load overwriting a newer draft: only
    /// the most recently started load may publish its result.
    private var loadGeneration = 0

    init(service: OctCLIService = OctCLIService(), snapshot: ConfigurationSnapshot? = nil) {
        self.service = service
        self.snapshot = snapshot
    }

    var isRevertAvailable: Bool { snapshot != nil }

    /// True while the edited draft differs from the last loaded snapshot;
    /// drives the pinned save bar at the bottom of the settings window.
    var hasUnsavedChanges: Bool {
        guard let draft, let snapshot else { return false }
        return draft != ConfigurationDraft(snapshot: snapshot)
    }

    /// Silent re-read used when surfaces open (popover, settings window).
    /// Keeps the last known configuration on failure.
    @discardableResult
    func reload() async -> ConfigurationSnapshot? {
        let generation = loadGeneration + 1
        loadGeneration = generation
        do {
            let fresh = try await service.fetchConfigurationSnapshot()
            guard generation == loadGeneration else { return snapshot }
            snapshot = fresh
            return fresh
        } catch {
            guard generation == loadGeneration else { return snapshot }
            feedback = .error(error.localizedDescription)
            return snapshot
        }
    }

    /// User-triggered load in settings: refreshes snapshot and draft with
    /// explicit feedback.
    func loadDraft() async {
        let generation = loadGeneration + 1
        loadGeneration = generation
        isLoading = true
        feedback = nil
        defer { isLoading = false }
        do {
            let fresh = try await service.fetchConfigurationSnapshot()
            guard generation == loadGeneration else { return }
            snapshot = fresh
            draft = ConfigurationDraft(snapshot: fresh)
            feedback = .success("Loaded configuration.")
        } catch {
            guard generation == loadGeneration else { return }
            feedback = .error(error.localizedDescription)
        }
    }

    func saveDraft() async {
        guard let pendingDraft = draft, pendingDraft.hasEnabledTool else {
            feedback = .warning("Select at least one provider.")
            return
        }
        isSaving = true
        defer { isSaving = false }
        do {
            try await service.saveConfiguration(pendingDraft.updatePayload())
            // Adopt the freshly saved state so the draft baseline matches.
            let fresh = try await service.fetchConfigurationSnapshot()
            snapshot = fresh
            draft = ConfigurationDraft(snapshot: fresh)
            feedback = .success("Saved.")
        } catch {
            feedback = .error(error.localizedDescription)
        }
    }

    func revertDraft() {
        guard let snapshot else { return }
        draft = ConfigurationDraft(snapshot: snapshot)
        feedback = .informational("Reverted to the last loaded configuration.")
    }
}
