import SwiftUI

struct SettingsView: View {
    @State private var selectedTab: SettingsTab = .providers
    @ObservedObject var configurationStore: ConfigurationStore
    @StateObject private var feedbackStore = FeedbackStore(
        service: FeedbackService(endpoint: FeedbackService.configuredEndpoint())
    )

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            settingsHeader
            settingsTabs
            unsavedChangesFooter
        }
        .padding(16)
        .frame(minWidth: 640, idealWidth: 640, maxWidth: 640, minHeight: 480, alignment: .topLeading)
        .overlay {
            if selectedTab != .about && configurationStore.isLoading && configurationStore.draft == nil {
                ProgressView("Loading configuration")
            }
        }
        .onAppear {
            // Reload-on-open policy: picks up external (CLI) config changes.
            // The draft itself lives in the shared store, so closing and
            // reopening the window never discards unsaved edits. The version
            // check runs alongside it so the per-provider update buttons are
            // ready by the time the Providers rows render.
            Task {
                await configurationStore.loadDraft()
                await configurationStore.checkAgentVersions()
                await configurationStore.loadMenubarDaemonState()
            }
        }
        .onDisappear {
            // The run-outcome chips (Updated!/Latest/Failed) describe the
            // last run for this viewing session: closing the window clears
            // them so the next open starts from plain version strings.
            configurationStore.settingsWindowDidClose()
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

            SettingsConfigurationTab(
                selectedTab: selectedTab,
                configDraft: $configurationStore.draft,
                isLoading: configurationStore.isLoading,
                releaseMonitor: .shared,
                feedbackStore: feedbackStore,
                feedback: configurationStore.feedback,
                toolUpdateStates: configurationStore.toolUpdateStates,
                isAgentUpdating: configurationStore.isAgentUpdating,
                singleUpdateBinaries: configurationStore.singleUpdateBinaries,
                agentUpdateProgress: configurationStore.agentUpdateProgress,
                versionChecks: configurationStore.versionChecks,
                failureReport: configurationStore.failureReport,
                onDraftChange: markConfigurationChanged,
                onLoad: { Task { await configurationStore.loadDraft() } },
                onRunAgentUpdate: { installMissing in
                    Task { await configurationStore.runAgentUpdateNow(installMissing: installMissing) }
                },
                onUpdateProvider: { binaryName in
                    Task { await configurationStore.runAgentUpdateNow(installMissing: false, onlyBinary: binaryName) }
                },
                onDismissFailureReport: {
                    configurationStore.dismissFailureReport()
                },
                menubarDaemonState: configurationStore.menubarDaemonState,
                onMenubarDaemonToggle: { enabled in
                    Task { await configurationStore.setMenubarDaemonEnabled(enabled) }
                }
            )
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
