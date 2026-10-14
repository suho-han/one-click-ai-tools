import SwiftUI

/// Providers section of the settings tab: the enable toggles, priority
/// arrows, per-provider update buttons and version chips, and the manual
/// agent-update run with its schedule — all inside one card. Extracted from
/// SettingsConfigurationTab; state lives in the shared draft.
struct SettingsProvidersSection: View {
    @Binding var configDraft: ConfigurationDraft?
    let onDraftChange: () -> Void
    let toolUpdateStates: [String: ToolUpdateState]
    let isAgentUpdating: Bool
    let singleUpdateBinaries: Set<String>
    let agentUpdateProgress: AgentUpdateProgress?
    let versionChecks: [String: ToolVersionCheckState]
    let onRunAgentUpdate: (Bool) -> Void
    let onUpdateProvider: (String) -> Void

    @State private var installPromptTools: [ConfigTool]?

    private static let nodeDownloadURL = URL(string: "https://nodejs.org/en/download")!

    var body: some View {
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
    }}
