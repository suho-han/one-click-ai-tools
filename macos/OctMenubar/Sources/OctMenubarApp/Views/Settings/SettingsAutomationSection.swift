import SwiftUI

/// Automation section of the settings tab: session-refresh schedule plus
/// the general app behavior card. Extracted from
/// SettingsConfigurationTab; the draft-backed toggles share the tab's save
/// flow.
struct SettingsAutomationSection: View {
    @Binding var configDraft: ConfigurationDraft?
    let onDraftChange: () -> Void
    let menubarDaemonState: MenubarDaemonState?
    let onMenubarDaemonToggle: (Bool) -> Void

    var body: some View {
        sessionRefreshSection
        generalSection
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


}
