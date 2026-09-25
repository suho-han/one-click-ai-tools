import SwiftUI

struct SettingsView: View {
    @State private var selectedTab: SettingsTab = .configuration
    @State private var lastActionFeedback: SettingsFeedback?
    @ObservedObject var configurationStore: ConfigurationStore

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            settingsHeader
            settingsTabs
            unsavedChangesFooter
        }
        .padding(16)
        .frame(minWidth: 640, idealWidth: 640, maxWidth: 640, minHeight: 480, alignment: .topLeading)
        .onAppear {
            // Reload-on-open policy: picks up external (CLI) config changes.
            // The draft itself lives in the shared store, so closing and
            // reopening the window never discards unsaved edits.
            Task { await configurationStore.loadDraft() }
        }
    }

    private var settingsHeader: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Settings")
                .font(.system(size: 19, weight: .semibold))
            Text(selectedTab.summary)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(.secondary)
        }
    }

    private var settingsTabs: some View {
        VStack(alignment: .leading, spacing: 0) {
            SettingsTabSelector(selection: $selectedTab)

            Group {
                switch selectedTab {
                case .configuration:
                    SettingsConfigurationTab(
                        configDraft: $configurationStore.draft,
                        isLoading: configurationStore.isLoading,
                        feedback: configurationStore.feedback,
                        onDraftChange: markConfigurationChanged,
                        onLoad: { Task { await configurationStore.loadDraft() } }
                    )
                case .tools:
                    SettingsToolsTab(feedback: lastActionFeedback, onAction: runAction)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }

    /// Pinned to the window bottom (outside the scrolling tab content) and
    /// shown only while the draft differs from the loaded snapshot.
    @ViewBuilder
    private var unsavedChangesFooter: some View {
        if configurationStore.hasUnsavedChanges {
            VStack(spacing: 10) {
                Divider()
                HStack(spacing: 8) {
                    Image(systemName: "pencil.circle")
                        .foregroundStyle(Color.accentColor)
                    Text("Unsaved changes")
                        .font(.system(size: 12, weight: .medium))

                    if configurationStore.isSaving {
                        ProgressView()
                            .controlSize(.small)
                    }

                    Spacer(minLength: 0)

                    Button(action: revertConfiguration) {
                        Label("Revert", systemImage: "arrow.uturn.backward")
                    }
                    .buttonStyle(.bordered)
                    .disabled(configurationStore.isSaving)

                    Button(action: { Task { await configurationStore.saveDraft() } }) {
                        Label("Save changes", systemImage: "checkmark.circle")
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(
                        configurationStore.isSaving
                            || !(configurationStore.draft?.hasEnabledTool ?? false)
                    )
                }
            }
        }
    }

    private func runAction(_ action: OctMenubarAction) {
        // Terminal launches run off the main thread; the window stays
        // responsive and only the feedback line updates.
        Task {
            do {
                try await OctCLIService().run(action: action)
                lastActionFeedback = .success("Launched \(action.settingsTitle) in Terminal.")
            } catch {
                lastActionFeedback = .error(error.localizedDescription)
            }
        }
    }

    private func revertConfiguration() {
        configurationStore.revertDraft()
    }

    private func markConfigurationChanged() {
        guard let configDraft = configurationStore.draft, let loadedConfig = configurationStore.snapshot else {
            return
        }

        // Unsaved edits are surfaced by the pinned footer, so the inline
        // feedback only carries actionable problems (e.g. no provider left).
        if configDraft == ConfigurationDraft(snapshot: loadedConfig) {
            configurationStore.feedback = nil
        } else if !configDraft.hasEnabledTool {
            configurationStore.feedback = .warning("Select at least one provider.")
        } else {
            configurationStore.feedback = nil
        }
    }
}

private extension OctMenubarAction {
    var settingsTitle: String {
        switch self {
        case .openUsage:
            return "Open usage"
        case .openMonitor:
            return "Open monitor"
        case .runAlertCheck:
            return "Run alert"
        case .runSessionRefresh:
            return "Session refresh"
        }
    }
}
