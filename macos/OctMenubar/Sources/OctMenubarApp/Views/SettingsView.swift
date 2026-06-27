import SwiftUI

struct SettingsView: View {
    @AppStorage(MenubarPreferences.useProviderAccentColorsKey) private var useProviderAccentColors = true
    @State private var lastActionMessage: String?
    @State private var loadedConfig: ConfigurationSnapshot?
    @State private var configDraft: ConfigurationDraft?
    @State private var configMessage: String?
    @State private var isConfigLoading = false
    @State private var isConfigSaving = false

    private let service = OctCLIService()

    var body: some View {
        Form {
            Section("Appearance") {
                Toggle("Use provider accent colors", isOn: $useProviderAccentColors)
                Text("Provider cards keep status colors for OK/WARN/ERROR and add brand-style accents to labels and chips.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }

            Section("Configuration") {
                configurationSection
            }

            Section("CLI Actions") {
                Text("These actions still launch Terminal, but they are moved out of the menubar popover.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)

                HStack(spacing: 10) {
                    settingsActionButton(title: "Open usage", systemImage: "chart.bar", action: .openUsage)
                    settingsActionButton(title: "Open monitor", systemImage: "waveform.path.ecg", action: .openMonitor)
                }

                HStack(spacing: 10) {
                    settingsActionButton(title: "Run alert", systemImage: "bell.badge", action: .runAlertCheck)
                    settingsActionButton(title: "Session refresh", systemImage: "key.horizontal", action: .runSessionRefresh)
                }

                if let lastActionMessage, !lastActionMessage.isEmpty {
                    Text(lastActionMessage)
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .formStyle(.grouped)
        .padding(16)
        .frame(width: 560)
        .onAppear {
            loadConfiguration()
        }
    }

    @ViewBuilder
    private var configurationSection: some View {
        if isConfigLoading && configDraft == nil {
            ProgressView("Loading configuration")
        } else if configDraft == nil {
            Button {
                loadConfiguration()
            } label: {
                Label("Load configuration", systemImage: "arrow.clockwise")
            }
        } else {
            configEditor
        }

        if let configMessage, !configMessage.isEmpty {
            Text(configMessage)
                .font(.footnote)
                .foregroundStyle(.secondary)
        }
    }

    private var configEditor: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(configDraft?.configFile ?? "")
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
                .lineLimit(2)

            providerToggles

            Picker("Usage display mode", selection: usageModeBinding) {
                ForEach(UsageDisplayMode.allCases) { mode in
                    Text(mode.label).tag(mode)
                }
            }
            .pickerStyle(.segmented)

            Toggle("Session refresh", isOn: sessionRefreshEnabledBinding)

            Picker("Session refresh interval", selection: sessionRefreshIntervalBinding) {
                Text("Daily").tag("daily")
                Text("Weekly").tag("weekly")
            }

            Stepper(value: sessionRefreshHourBinding, in: 0...23) {
                Text("Session refresh hour: \(configDraft?.sessionRefreshHour ?? 0):00")
            }

            HStack(spacing: 10) {
                Button {
                    saveConfiguration()
                } label: {
                    Label("Save", systemImage: "checkmark.circle")
                }
                .disabled(isConfigSaving || !(configDraft?.hasEnabledTool ?? false))

                Button {
                    revertConfiguration()
                } label: {
                    Label("Revert", systemImage: "arrow.uturn.backward")
                }
                .disabled(isConfigSaving || loadedConfig == nil)

                if isConfigSaving {
                    ProgressView()
                        .controlSize(.small)
                }
            }
        }
    }

    private var providerToggles: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Enabled providers")
                .font(.system(size: 12, weight: .semibold))

            ForEach(configDraft?.tools ?? []) { tool in
                Toggle(tool.name, isOn: toolEnabledBinding(tool.binaryName))
            }

            if configDraft?.hasEnabledTool == false {
                Text("Select at least one provider.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private func settingsActionButton(title: String, systemImage: String, action: OctMenubarAction) -> some View {
        Button {
            do {
                try service.run(action: action)
                lastActionMessage = "Launched \(title) in Terminal."
            } catch {
                lastActionMessage = error.localizedDescription
            }
        } label: {
            Label(title, systemImage: systemImage)
                .font(.system(size: 12, weight: .semibold))
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 12)
                .padding(.vertical, 10)
                .background(
                    RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .fill(Color(nsColor: .controlBackgroundColor))
                )
        }
        .buttonStyle(.plain)
    }

    private var usageModeBinding: Binding<UsageDisplayMode> {
        Binding(
            get: { configDraft?.usageDisplayMode ?? .remaining },
            set: { configDraft?.usageDisplayMode = $0 }
        )
    }

    private var sessionRefreshEnabledBinding: Binding<Bool> {
        Binding(
            get: { configDraft?.sessionRefreshEnabled ?? false },
            set: { configDraft?.sessionRefreshEnabled = $0 }
        )
    }

    private var sessionRefreshIntervalBinding: Binding<String> {
        Binding(
            get: { configDraft?.sessionRefreshInterval ?? "daily" },
            set: { configDraft?.sessionRefreshInterval = $0 }
        )
    }

    private var sessionRefreshHourBinding: Binding<Int> {
        Binding(
            get: { configDraft?.sessionRefreshHour ?? 9 },
            set: { configDraft?.sessionRefreshHour = $0 }
        )
    }

    private func toolEnabledBinding(_ binaryName: String) -> Binding<Bool> {
        Binding(
            get: {
                configDraft?.tools.first(where: { $0.binaryName == binaryName })?.enabled ?? false
            },
            set: { enabled in
                configDraft?.setTool(binaryName, enabled: enabled)
            }
        )
    }

    private func loadConfiguration() {
        isConfigLoading = true
        configMessage = nil
        do {
            let snapshot = try service.fetchConfigurationSnapshot()
            loadedConfig = snapshot
            configDraft = ConfigurationDraft(snapshot: snapshot)
            configMessage = "Loaded configuration."
        } catch {
            configMessage = error.localizedDescription
        }
        isConfigLoading = false
    }

    private func saveConfiguration() {
        guard let configDraft, configDraft.hasEnabledTool else {
            configMessage = "Select at least one provider."
            return
        }

        isConfigSaving = true
        do {
            try service.saveConfiguration(configDraft.updatePayload())
            let snapshot = try service.fetchConfigurationSnapshot()
            loadedConfig = snapshot
            self.configDraft = ConfigurationDraft(snapshot: snapshot)
            configMessage = "Saved."
        } catch {
            configMessage = error.localizedDescription
        }
        isConfigSaving = false
    }

    private func revertConfiguration() {
        guard let loadedConfig else {
            return
        }
        configDraft?.revert(to: loadedConfig)
        configMessage = "Reverted."
    }
}
