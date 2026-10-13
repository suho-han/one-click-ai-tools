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

    @State private var installPromptTools: [ConfigTool]?
    @AppStorage(MenubarPreferences.sharedAlertThresholdKey) private var sharesAlertThreshold = false

    private static let nodeDownloadURL = URL(string: "https://nodejs.org/en/download")!

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
                providerSection
            case .display:
                appearanceSection
                displaySection
            case .alerts:

                // Alerts live in their own file: threshold, cooldown, and quiet
                // timer controls are a self-contained cluster.
                SettingsAlertSection(configDraft: $configDraft, onDraftChange: onDraftChange)
            case .automation:
                sessionRefreshSection
                generalSection
            case .about:
                SettingsAboutTab(releaseMonitor: releaseMonitor, feedbackStore: feedbackStore)
            }
        }
    }

    private var providerSection: some View {
        SettingsSectionCard(
            title: "Providers",
            systemImage: "person.2",
            description: "Choose usage providers and change execution priority with the arrow controls."
        ) {
            VStack(alignment: .leading, spacing: 8) {
                LabeledContent("Configuration file") {
                    Text(configDraft?.configFile ?? "")
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                        .multilineTextAlignment(.trailing)
                        .textSelection(.enabled)
                }

                Divider()

                ForEach(Array((configDraft?.tools ?? []).enumerated()), id: \.element.id) { index, tool in
                    HStack(spacing: 8) {
                        Toggle(isOn: toolEnabledBinding(tool.binaryName)) {
                            HStack(spacing: 6) {
                                Text(tool.name)
                                toolVersionChip(tool, state: toolUpdateStates[tool.binaryName])
                            }
                        }

                        providerUpdateButton(tool)

                        if toolUpdateStates[tool.binaryName] == .npmMissing {
                            nodeInstallLink(for: tool)
                        }

                        Spacer(minLength: 8)

                        Text("#\(index + 1)")
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(.secondary)
                            .frame(width: 28, alignment: .trailing)

                        Button {
                            moveTool(tool.binaryName, by: -1)
                        } label: {
                            Image(systemName: "chevron.up")
                        }
                        .buttonStyle(.borderless)
                        .disabled(index == 0)
                        .accessibilityLabel("Move \(tool.name) up")

                        Button {
                            moveTool(tool.binaryName, by: 1)
                        } label: {
                            Image(systemName: "chevron.down")
                        }
                        .buttonStyle(.borderless)
                        .disabled(index == (configDraft?.tools.count ?? 0) - 1)
                        .accessibilityLabel("Move \(tool.name) down")
                    }
                }

                if configDraft?.hasEnabledTool == false {
                    SettingsFeedbackMessage(
                        feedback: .warning("Select at least one provider."),
                        compact: true
                    )
                }

                Divider()

                agentUpdateSection
            }
        }
    }

    /// Installed-version chip next to a provider name. nil (older oct without
    /// `--probe-versions`) renders nothing; "" means probed but not installed.
    /// While a background agent update runs, the live state wins: Updating…
    /// (with a spinner), then Updated!/Latest for a beat, then back to the
    /// (new) version once the store clears the transient state.
    @ViewBuilder
    private func toolVersionChip(_ tool: ConfigTool, state: ToolUpdateState?) -> some View {
        if let state {
            updateStateChip(state, toolName: tool.name)
        } else if let version = tool.version {
            Text(version.isEmpty ? "❌ not installed" : "v\(version)")
                .font(.system(size: 10, weight: .medium).monospacedDigit())
                .foregroundStyle(version.isEmpty ? Color(nsColor: .systemOrange) : Color.secondary)
                .padding(.horizontal, 5)
                .padding(.vertical, 1)
                .background(
                    Capsule(style: .continuous)
                        .fill(Color(nsColor: .quaternaryLabelColor).opacity(0.35))
                )
                .help(version.isEmpty ? "\(tool.name) is not installed; background updates skip it." : "\(tool.name) \(version)")
        }
    }

    private func updateStateChip(_ state: ToolUpdateState, toolName: String) -> some View {
        let label: String
        let color: Color
        var showsSpinner = false
        var helpText = ""
        switch state {
        case .updating:
            label = "Updating…"
            color = Color.accentColor
            showsSpinner = true
            helpText = "\(toolName) is being updated in the background."
        case .updated:
            label = "Updated!"
            color = Color(nsColor: .systemGreen)
            helpText = "\(toolName) was updated successfully."
        case .upToDate:
            label = "Latest"
            color = Color(nsColor: .systemGreen)
            helpText = "\(toolName) is already up to date."
        case .failed(let message):
            label = "Failed"
            color = Color(nsColor: .systemRed)
            helpText = "\(toolName) update failed: \(message)"
        case .npmMissing:
            label = "npm missing"
            color = Color(nsColor: .systemOrange)
            helpText = "npm was not found on PATH — install Node.js (link on the row) to update \(toolName)."
        }
        return HStack(spacing: 4) {
            if showsSpinner {
                ProgressView()
                    .controlSize(.mini)
            }
            Text(label)
        }
        .font(.system(size: 10, weight: .medium).monospacedDigit())
        .foregroundStyle(color)
        .padding(.horizontal, 5)
        .padding(.vertical, 1)
        .background(
            Capsule(style: .continuous)
                .fill(color.opacity(0.14))
        )
        .help(helpText)
        .animation(.easeInOut(duration: 0.18), value: state)
    }

    /// Download affordance immediately right of the version chip: shown only
    /// when the version check found a known newer version. Managers without
    /// a query API (native updaters, install scripts) never show it — the
    /// run-all button below updates them. Clicking updates only this
    /// provider in the background; the chip takes over with Updating… while
    /// it runs. A sibling of the Toggle (not inside its label) so the click
    /// can never flip the enable checkbox. Row runs fly in parallel: another
    /// provider updating never disables this button (its own run hides it
    /// via the chip); only a run-all sweep — which owns every enabled
    /// provider — does.
    @ViewBuilder
    private func providerUpdateButton(_ tool: ConfigTool) -> some View {
        if isProviderUpdateOffered(tool) {
            Button {
                onUpdateProvider(tool.binaryName)
            } label: {
                Image(systemName: "arrow.down.circle")
                    .font(.system(size: 11, weight: .medium))
            }
            .buttonStyle(.borderless)
            .disabled(isAgentUpdating)
            .help(updateButtonHelp(tool))
            .accessibilityLabel("Update \(tool.name)")
        }
    }

    private func isProviderUpdateOffered(_ tool: ConfigTool) -> Bool {
        guard tool.supportsUpdate, let version = tool.version, !version.isEmpty else { return false }
        guard toolUpdateStates[tool.binaryName] == nil else { return false }
        return versionChecks[tool.binaryName]?.offersUpdate == true
    }

    private func updateButtonHelp(_ tool: ConfigTool) -> String {
        versionChecks[tool.binaryName]?.updateHelp ?? "Check and update this provider"
    }

    /// Manual run + schedule controls for agent-update, living inside the
    /// Providers card: the schedule drives `oct agent-update` (the update of
    /// exactly these providers) via the platform scheduler. The manual run
    /// happens in this app's background — no Terminal — with per-provider
    /// progress on the version chips above. Enabled providers whose probe
    /// says "not installed" get one confirmation dialog first.
    private var agentUpdateSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 10) {
                Button {
                    requestAgentUpdateRun()
                } label: {
                    Label(
                        isAgentUpdating ? "Updating providers…" : "Run agent-update now",
                        systemImage: "arrow.triangle.down.circle"
                    )
                }
                .buttonStyle(.bordered)
                .disabled(
                    isAgentUpdating
                        || !singleUpdateBinaries.isEmpty
                        || (configDraft?.hasEnabledUpdateTool ?? false) == false
                )
                .animation(.easeInOut(duration: 0.18), value: isAgentUpdating)

                if let progress = agentUpdateProgress {
                    Text("\(progress.current)/\(progress.total)")
                        .font(.system(size: 11, weight: .medium).monospacedDigit())
                        .foregroundStyle(.secondary)
                }

                Text("Updates every enabled provider in the background.")
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
            }
            .alert(
                "Install missing providers?",
                isPresented: Binding(
                    get: { installPromptTools != nil },
                    set: { if !$0 { installPromptTools = nil } }
                ),
                presenting: installPromptTools
            ) { tools in
                Button("Install and Update") {
                    onRunAgentUpdate(true)
                    installPromptTools = nil
                }
                Button("Update Installed Only") {
                    onRunAgentUpdate(false)
                    installPromptTools = nil
                }
                Button("Cancel", role: .cancel) {
                    installPromptTools = nil
                }
            } message: { tools in
                Text("These providers are not installed: \(tools.map(\.name).joined(separator: ", ")). Install them with their default method, then update?")
            }

            Picker("Update schedule", selection: agentUpdateScheduleBinding) {
                Text("Off").tag(AgentUpdateSchedule.offTag)
                ForEach(SessionRefreshIntervalOption.all) { option in
                    Text(option.label).tag(option.value)
                }
                if configDraft?.agentUpdateSchedule.isCustom == true {
                    Text("On (custom)").tag(AgentUpdateSchedule.customTag)
                }
            }

            if configDraft?.agentUpdateSchedule.usesHour == true {
                Stepper(value: agentUpdateHourBinding, in: 0...23) {
                    Text("Run at \(configDraft?.agentUpdateSchedule.hour ?? 9):00")
                }
            }
        }
    }

    /// Enabled providers the last probe reported as not installed (version
    /// "") need one confirmation before the background run installs them;
    /// nil versions (oct too old to probe) count as installed.
    private func requestAgentUpdateRun() {
        let missing = (configDraft?.tools ?? []).filter { $0.enabled && $0.supportsUpdate && $0.version == "" }
        guard !missing.isEmpty else {
            onRunAgentUpdate(false)
            return
        }
        installPromptTools = missing
    }

    /// Opens the Node.js download page: the fix for every npm-family update
    /// that could not run because npm itself is missing.
    private func nodeInstallLink(for tool: ConfigTool) -> some View {
        Link(destination: Self.nodeDownloadURL) {
            Image(systemName: "arrow.up.right.square")
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(Color(nsColor: .systemOrange))
        }
        .buttonStyle(.borderless)
        .help("npm was not found. Open nodejs.org to install Node.js (includes npm), then run the update again for \(tool.name).")
        .accessibilityLabel("Install Node.js to update \(tool.name)")
    }

    private var agentUpdateScheduleBinding: Binding<String> {
        Binding(
            get: { configDraft?.agentUpdateSchedule.pickerValue ?? AgentUpdateSchedule.offTag },
            set: {
                configDraft?.agentUpdateSchedule.setPickerValue($0)
                onDraftChange()
            }
        )
    }

    private var agentUpdateHourBinding: Binding<Int> {
        Binding(
            get: { configDraft?.agentUpdateSchedule.hour ?? 9 },
            set: {
                configDraft?.agentUpdateSchedule.hour = $0
                onDraftChange()
            }
        )
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

    private var sessionRefreshSection: some View {
        SettingsSectionCard(
            title: "Session refresh",
            systemImage: "arrow.clockwise.circle",
            description: "Keep supported provider sessions current on a regular schedule."
        ) {
            VStack(alignment: .leading, spacing: 8) {
                Toggle("Refresh sessions automatically", isOn: sessionRefreshEnabledBinding)

                Picker("Refresh interval", selection: sessionRefreshIntervalBinding) {
                    ForEach(SessionRefreshIntervalOption.all) { option in
                        Text(option.label).tag(option.value)
                    }
                }

                if sessionRefreshUsesHour {
                    Stepper(value: sessionRefreshHourBinding, in: 0...23) {
                        Text("Refresh hour: \(configDraft?.sessionRefreshHour ?? 0):00")
                    }
                } else {
                    LabeledContent("Refresh hour") {
                        Text("Not used for sub-daily intervals")
                            .foregroundStyle(.secondary)
                    }
                }
            }
        }
    }

    /// Launch-at-login is app-level behavior applied immediately; usage
    /// analytics is part of the oct config draft and applies only on Save.
    private var generalSection: some View {
        SettingsSectionCard(
            title: "General",
            systemImage: "gearshape",
            description: "App behavior."
        ) {
            VStack(alignment: .leading, spacing: 6) {
                Toggle("Launch at login", isOn: menubarDaemonBinding)
                    .disabled(menubarDaemonState == nil)
                Text(menubarDaemonCaption)
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)

                Toggle("Share usage analytics", isOn: telemetryEnabledBinding)
                Text("Uses a random installation ID to measure commands and return visits. No prompts or credentials. Apply with Save; environment opt-outs still take priority.")
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
            }
        }
    }

    private var menubarDaemonCaption: String {
        guard menubarDaemonState != nil else {
            return "The installed oct binary does not report launch-at-login support."
        }
        return "Start the menubar automatically when you log in."
    }

    private var menubarDaemonBinding: Binding<Bool> {
        Binding(
            get: { menubarDaemonState?.enabled ?? false },
            set: { onMenubarDaemonToggle($0) }
        )
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

    private var telemetryEnabledBinding: Binding<Bool> {
        Binding(
            get: { configDraft?.telemetryEnabled ?? true },
            set: {
                configDraft?.telemetryEnabled = $0
                onDraftChange()
            }
        )
    }

    private var sessionRefreshEnabledBinding: Binding<Bool> {
        Binding(
            get: { configDraft?.sessionRefreshEnabled ?? false },
            set: {
                configDraft?.sessionRefreshEnabled = $0
                onDraftChange()
            }
        )
    }

    private var sessionRefreshIntervalBinding: Binding<String> {
        Binding(
            get: { SessionRefreshIntervalOption.option(for: configDraft?.sessionRefreshInterval ?? "daily").value },
            set: {
                configDraft?.sessionRefreshInterval = $0
                onDraftChange()
            }
        )
    }

    private var sessionRefreshHourBinding: Binding<Int> {
        Binding(
            get: { configDraft?.sessionRefreshHour ?? 9 },
            set: {
                configDraft?.sessionRefreshHour = $0
                onDraftChange()
            }
        )
    }

    private var sessionRefreshUsesHour: Bool {
        SessionRefreshIntervalOption.usesHour(configDraft?.sessionRefreshInterval ?? "daily")
    }


    private func toolEnabledBinding(_ binaryName: String) -> Binding<Bool> {
        Binding(
            get: {
                configDraft?.tools.first(where: { $0.binaryName == binaryName })?.enabled ?? false
            },
            set: { enabled in
                configDraft?.setTool(binaryName, enabled: enabled)
                onDraftChange()
            }
        )
    }
    private func moveTool(_ binaryName: String, by offset: Int) {
        configDraft?.moveTool(binaryName, by: offset)
        onDraftChange()
    }
}
