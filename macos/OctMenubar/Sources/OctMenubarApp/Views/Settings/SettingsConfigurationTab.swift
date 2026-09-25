import SwiftUI

struct SettingsConfigurationTab: View {
    @AppStorage(MenubarPreferences.useProviderAccentColorsKey) private var useProviderAccentColors = true
    @Binding var configDraft: ConfigurationDraft?

    let isLoading: Bool
    let isSaving: Bool
    let isRevertAvailable: Bool
    let feedback: SettingsFeedback?
    let onDraftChange: () -> Void
    let onLoad: () -> Void
    let onSave: () -> Void
    let onRevert: () -> Void

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 12) {
                configurationContent

                if let feedback {
                    SettingsFeedbackMessage(feedback: feedback)
                }
            }
            .padding(.vertical, 12)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    @ViewBuilder
    private var configurationContent: some View {
        if isLoading && configDraft == nil {
            SettingsSectionCard(
                title: "Configuration",
                systemImage: "arrow.triangle.2.circlepath",
                description: "Reading the current oct configuration."
            ) {
                ProgressView("Loading configuration")
            }
        } else if configDraft == nil {
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
            providerSection
            appearanceSection
            displaySection
            alertSection
            sessionRefreshSection
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
                        Toggle(tool.name, isOn: toolEnabledBinding(tool.binaryName))

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

    private var alertSection: some View {
        SettingsSectionCard(
            title: "Alerts",
            systemImage: "bell.badge",
            description: "Set global usage notification preferences."
        ) {
            VStack(alignment: .leading, spacing: 8) {
                Toggle("Enable usage alerts", isOn: alertEnabledBinding)

                Stepper(value: alertThresholdPercentBinding, in: 1...100, step: 1) {
                    Text("Alert threshold: \(configDraft?.alert.thresholdPercent ?? 80, specifier: "%.0f")%")
                }

                Stepper(value: alertCriticalPercentBinding, in: 1...100, step: 1) {
                    Text("Critical threshold: \(configDraft?.alert.criticalPercent ?? 98, specifier: "%.0f")%")
                }

                Stepper(value: alertCooldownMinutesBinding, in: 1...1_440, step: 1) {
                    Text("Cooldown: \(configDraft?.alert.cooldownMinutes ?? 360) minutes")
                }

                TextField("Quiet hours", text: alertQuietHoursBinding)
                TextField("Timezone", text: alertTimezoneBinding)

                Divider()

                Text("Window thresholds")
                    .font(.system(size: 12, weight: .medium))

                Stepper(value: alertDefaultThresholdBinding, in: 1...100, step: 1) {
                    Text("Default: \(configDraft?.alert.thresholds.defaultThreshold ?? 80, specifier: "%.0f")%")
                }

                Stepper(value: alertFiveHoursThresholdBinding, in: 1...100, step: 1) {
                    Text("5h: \(configDraft?.alert.thresholds.fiveHours ?? 80, specifier: "%.0f")%")
                }

                Stepper(value: alertSevenDaysThresholdBinding, in: 1...100, step: 1) {
                    Text("7d: \(configDraft?.alert.thresholds.sevenDays ?? 80, specifier: "%.0f")%")
                }
            }
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

    private var saveActions: some View {
        HStack(spacing: 8) {
            Button(action: onSave) {
                Label("Save changes", systemImage: "checkmark.circle")
            }
            .buttonStyle(.borderedProminent)
            .disabled(isSaving || !(configDraft?.hasEnabledTool ?? false))

            Button(action: onRevert) {
                Label("Revert", systemImage: "arrow.uturn.backward")
            }
            .buttonStyle(.bordered)
            .disabled(isSaving || !isRevertAvailable)

            if isSaving {
                ProgressView()
                    .controlSize(.small)
            }

            Spacer(minLength: 0)
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

    private var alertEnabledBinding: Binding<Bool> {
        Binding(
            get: { configDraft?.alert.enabled ?? false },
            set: {
                configDraft?.alert.enabled = $0
                onDraftChange()
            }
        )
    }

    private var alertThresholdPercentBinding: Binding<Double> {
        alertBinding(\.thresholdPercent)
    }

    private var alertCriticalPercentBinding: Binding<Double> {
        alertBinding(\.criticalPercent)
    }

    private var alertCooldownMinutesBinding: Binding<Int> {
        Binding(
            get: { configDraft?.alert.cooldownMinutes ?? 360 },
            set: {
                configDraft?.alert.cooldownMinutes = $0
                onDraftChange()
            }
        )
    }

    private var alertQuietHoursBinding: Binding<String> {
        alertBinding(\.quietHours)
    }

    private var alertTimezoneBinding: Binding<String> {
        alertBinding(\.timezone)
    }

    private var alertDefaultThresholdBinding: Binding<Double> {
        alertThresholdBinding(\.defaultThreshold)
    }

    private var alertFiveHoursThresholdBinding: Binding<Double> {
        alertThresholdBinding(\.fiveHours)
    }

    private var alertSevenDaysThresholdBinding: Binding<Double> {
        alertThresholdBinding(\.sevenDays)
    }

    private func alertBinding<Value>(_ keyPath: WritableKeyPath<AlertSettings, Value>) -> Binding<Value> {
        Binding(
            get: { configDraft?.alert[keyPath: keyPath] ?? AlertSettings.goDefaults[keyPath: keyPath] },
            set: {
                configDraft?.alert[keyPath: keyPath] = $0
                onDraftChange()
            }
        )
    }

    private func alertThresholdBinding(_ keyPath: WritableKeyPath<AlertThresholds, Double>) -> Binding<Double> {
        Binding(
            get: { configDraft?.alert.thresholds[keyPath: keyPath] ?? AlertSettings.goDefaults.thresholds[keyPath: keyPath] },
            set: {
                configDraft?.alert.thresholds[keyPath: keyPath] = $0
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
