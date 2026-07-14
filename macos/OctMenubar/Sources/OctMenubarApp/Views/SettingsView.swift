import SwiftUI

struct SettingsView: View {
    @AppStorage(MenubarPreferences.useProviderAccentColorsKey) private var useProviderAccentColors = true
    @State private var selectedTab: SettingsTab = .general
    @State private var lastActionFeedback: SettingsFeedback?
    @State private var loadedConfig: ConfigurationSnapshot?
    @State private var configDraft: ConfigurationDraft?
    @State private var configFeedback: SettingsFeedback?
    @State private var isConfigLoading = false
    @State private var isConfigSaving = false

    private let service = OctCLIService()

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            settingsHeader
            settingsTabs
        }
        .padding(16)
        .frame(minWidth: 640, idealWidth: 640, maxWidth: 640, minHeight: 480, alignment: .topLeading)
        .onAppear(perform: loadConfiguration)
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
                case .general:
                    SettingsGeneralTab(useProviderAccentColors: $useProviderAccentColors)
                case .configuration:
                    SettingsConfigurationTab(
                        configDraft: $configDraft,
                        isLoading: isConfigLoading,
                        isSaving: isConfigSaving,
                        isRevertAvailable: loadedConfig != nil,
                        feedback: configFeedback,
                        onDraftChange: markConfigurationChanged,
                        onLoad: loadConfiguration,
                        onSave: saveConfiguration,
                        onRevert: revertConfiguration
                    )
                case .tools:
                    SettingsToolsTab(feedback: lastActionFeedback, onAction: runAction)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }

    private func runAction(_ action: OctMenubarAction) {
        do {
            try service.run(action: action)
            lastActionFeedback = .success("Launched \(action.settingsTitle) in Terminal.")
        } catch {
            lastActionFeedback = .error(error.localizedDescription)
        }
    }

    private func loadConfiguration() {
        isConfigLoading = true
        configFeedback = nil
        do {
            let snapshot = try service.fetchConfigurationSnapshot()
            loadedConfig = snapshot
            configDraft = ConfigurationDraft(snapshot: snapshot)
            configFeedback = .success("Loaded configuration.")
        } catch {
            configFeedback = .error(error.localizedDescription)
        }
        isConfigLoading = false
    }

    private func saveConfiguration() {
        guard let configDraft, configDraft.hasEnabledTool else {
            configFeedback = .warning("Select at least one provider.")
            return
        }

        isConfigSaving = true
        do {
            try service.saveConfiguration(configDraft.updatePayload())
            let snapshot = try service.fetchConfigurationSnapshot()
            loadedConfig = snapshot
            self.configDraft = ConfigurationDraft(snapshot: snapshot)
            configFeedback = .success("Saved.")
        } catch {
            configFeedback = .error(error.localizedDescription)
        }
        isConfigSaving = false
    }

    private func revertConfiguration() {
        guard let loadedConfig else {
            return
        }
        configDraft?.revert(to: loadedConfig)
        configFeedback = .informational("Reverted to the last loaded configuration.")
    }

    private func markConfigurationChanged() {
        guard let configDraft, let loadedConfig else {
            return
        }

        if configDraft == ConfigurationDraft(snapshot: loadedConfig) {
            configFeedback = nil
        } else if configDraft.hasEnabledTool {
            configFeedback = .informational("Unsaved changes.")
        } else {
            configFeedback = .warning("Select at least one provider.")
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
