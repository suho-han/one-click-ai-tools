import SwiftUI

struct SettingsConfigurationTab: View {
    let selectedTab: SettingsTab
    @AppStorage(MenubarPreferences.useProviderAccentColorsKey) private var useProviderAccentColors = true
    @Binding var configDraft: ConfigurationDraft?

    let isLoading: Bool
    @ObservedObject var releaseMonitor: ReleaseUpdateMonitor
    @ObservedObject var feedbackStore: FeedbackStore
    let feedback: SettingsFeedback?
    /// Live per-provider progress for the background agent update, keyed by
    /// binaryName; drives the version chip through Updating… → Updated!.
    let toolUpdateStates: [String: ToolUpdateState]
    /// True only while a run-all sweep is in flight — the only thing that
    /// disables every row's update button, since a sweep owns all enabled
    /// providers at once.
    let isAgentUpdating: Bool
    /// Providers with an in-flight row-level update run. Row updates fly in
    /// parallel, so sibling rows keep their buttons enabled; this only takes
    /// the run-all button off while any row run is airborne (the store would
    /// refuse it anyway — the disabled state makes that visible).
    let singleUpdateBinaries: Set<String>
    let agentUpdateProgress: AgentUpdateProgress?
    /// Latest-version knowledge per provider (agent-update --check), keyed by
    /// binaryName; decides which rows show the update button.
    let versionChecks: [String: ToolVersionCheckState]
    /// Why the last background run's providers failed, presented as an alert.
    /// Nil when there was nothing to explain or the alert was dismissed.
    let failureReport: AgentUpdateFailureReport?
    let onDraftChange: () -> Void
    let onLoad: () -> Void
    /// Runs the background agent update; the flag is the user's answer to
    /// the install-missing confirmation dialog.
    let onRunAgentUpdate: (Bool) -> Void
    /// Updates a single provider in the background (the row-level download
    /// button); the binaryName is the row the user clicked.
    let onUpdateProvider: (String) -> Void
    /// The failure alert was dismissed.
    let onDismissFailureReport: () -> Void
    /// Launch-at-login registration for this track's menubar daemon. Nil
    /// means unknown (not loaded yet, or the installed oct is too old to
    /// report it) — the toggle renders disabled.
    let menubarDaemonState: MenubarDaemonState?
    /// Toggles launch-at-login; the store settles the state and surfaces
    /// failures as feedback.
    let onMenubarDaemonToggle: (Bool) -> Void


    var body: some View {
        Group {
            if isLoading && configDraft == nil && selectedTab != .about {
                Color.clear
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView(.vertical) {
                    VStack(alignment: .leading, spacing: 12) {
                        configurationContent

                        if let feedback, selectedTab != .about {
                            SettingsFeedbackMessage(feedback: feedback)
                                .transition(.opacity)
                        }
                    }
                    .padding(.vertical, 12)
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
        .id(selectedTab)
        .alert(
            failureReport?.title ?? "",
            isPresented: Binding(
                get: { failureReport != nil },
                set: { if !$0 { onDismissFailureReport() } }
            ),
            presenting: failureReport
        ) { _ in
            Button("OK", role: .cancel) {}
        } message: { report in
            Text(report.message)
        }
    }

    @ViewBuilder
    private var configurationContent: some View {
        if configDraft == nil && selectedTab != .about {
            SettingsSectionCard(
                title: "Configuration",
                systemImage: "slider.horizontal.3",
                description: "Load the current oct configuration before making changes."
            ) {
                Button(action: onLoad) {
                    Label("Load configuration", systemImage: "arrow.clockwise")
                }
                .buttonStyle(.bordered)
            }
        } else {
            switch selectedTab {
            case .providers:
                SettingsProvidersSection(
                    configDraft: $configDraft,
                    onDraftChange: onDraftChange,
                    toolUpdateStates: toolUpdateStates,
                    isAgentUpdating: isAgentUpdating,
                    singleUpdateBinaries: singleUpdateBinaries,
                    agentUpdateProgress: agentUpdateProgress,
                    versionChecks: versionChecks,
                    onRunAgentUpdate: onRunAgentUpdate,
                    onUpdateProvider: onUpdateProvider
                )
            case .display:
                appearanceSection
                displaySection
            case .alerts:

                // Alerts live in their own file: threshold, cooldown, and quiet
                // timer controls are a self-contained cluster.
                SettingsAlertSection(configDraft: $configDraft, onDraftChange: onDraftChange)
            case .automation:
                SettingsAutomationSection(
                    configDraft: $configDraft,
                    onDraftChange: onDraftChange,
                    menubarDaemonState: menubarDaemonState,
                    onMenubarDaemonToggle: onMenubarDaemonToggle
                )
            case .about:
                SettingsAboutTab(releaseMonitor: releaseMonitor, feedbackStore: feedbackStore)
            }
        }
    }

    private var displaySection: some View {
        SettingsSectionCard(
            title: "Usage display",
            systemImage: "chart.bar",
            description: "Cards and menu bar always show remaining quota."
        ) {
            VStack(alignment: .leading, spacing: 8) {
                Picker("Menu bar display", selection: menubarTitleModeBinding) {
                    ForEach(MenubarTitleMode.allCases) { mode in
                        Text(mode.label).tag(mode)
                    }
                }
                .pickerStyle(.segmented)
            }
        }
    }

    private var appearanceSection: some View {
        SettingsSectionCard(
            title: "Appearance",
            systemImage: "paintpalette",
            description: "Use provider colors for names and metric chips while preserving semantic status colors."
        ) {
            Toggle("Use provider accent colors", isOn: $useProviderAccentColors)
        }
    }

    private var menubarTitleModeBinding: Binding<MenubarTitleMode> {
        Binding(
            get: { configDraft?.menubarTitleMode ?? .oct },
            set: {
                configDraft?.setMenubarTitleMode($0)
                onDraftChange()
            }
        )
    }


}
