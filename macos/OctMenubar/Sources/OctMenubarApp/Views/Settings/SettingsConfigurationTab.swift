import SwiftUI

struct SettingsConfigurationTab: View {
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
            usageSection
            sessionRefreshSection
            saveActions
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

    private var usageSection: some View {
        SettingsSectionCard(
            title: "Usage display",
            systemImage: "chart.bar",
            description: "Choose whether usage cards lead with remaining or used quota."
        ) {
            Picker("Usage display mode", selection: usageModeBinding) {
                ForEach(UsageDisplayMode.allCases) { mode in
                    Text(mode.label).tag(mode)
                }
            }
            .pickerStyle(.segmented)
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

    private var usageModeBinding: Binding<UsageDisplayMode> {
        Binding(
            get: { configDraft?.usageDisplayMode ?? .remaining },
            set: {
                configDraft?.usageDisplayMode = $0
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
