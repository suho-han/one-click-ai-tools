import SwiftUI

struct SettingsView: View {
    @AppStorage(MenubarPreferences.useProviderAccentColorsKey) private var useProviderAccentColors = true
    @State private var lastActionMessage: String?

    var body: some View {
        Form {
            Section("Appearance") {
                Toggle("Use provider accent colors", isOn: $useProviderAccentColors)
                Text("Provider cards keep status colors for OK/WARN/ERROR and add brand-style accents to labels and chips.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
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
        .frame(width: 520)
    }

    private func settingsActionButton(title: String, systemImage: String, action: OctMenubarAction) -> some View {
        Button {
            do {
                try OctCLIService().run(action: action)
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
}
