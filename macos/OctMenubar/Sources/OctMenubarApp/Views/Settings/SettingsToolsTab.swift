import SwiftUI

struct SettingsToolsTab: View {
    let feedback: SettingsFeedback?
    let onAction: (OctMenubarAction) -> Void

    private let columns = [
        GridItem(.flexible(minimum: 0, maximum: .infinity), spacing: 8),
        GridItem(.flexible(minimum: 0, maximum: .infinity), spacing: 8),
    ]

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 12) {
                SettingsSectionCard(
                    title: "Terminal tools",
                    systemImage: "terminal",
                    description: "These commands open in Terminal so interactive output stays available."
                ) {
                    LazyVGrid(columns: columns, alignment: .leading, spacing: 8) {
                        SettingsActionTile(
                            title: "Open usage",
                            description: "Inspect current provider usage.",
                            systemImage: "chart.bar"
                        ) {
                            onAction(.openUsage)
                        }

                        SettingsActionTile(
                            title: "Open monitor",
                            description: "Run one monitoring check.",
                            systemImage: "waveform.path.ecg"
                        ) {
                            onAction(.openMonitor)
                        }

                        SettingsActionTile(
                            title: "Run alert",
                            description: "Evaluate usage notification rules.",
                            systemImage: "bell.badge"
                        ) {
                            onAction(.runAlertCheck)
                        }

                        SettingsActionTile(
                            title: "Session refresh",
                            description: "Refresh supported provider sessions.",
                            systemImage: "key.horizontal"
                        ) {
                            onAction(.runSessionRefresh)
                        }
                    }
                }

                if let feedback {
                    SettingsFeedbackMessage(feedback: feedback)
                }
            }
            .padding(.vertical, 12)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}

private struct SettingsActionTile: View {
    let title: String
    let description: String
    let systemImage: String
    let action: () -> Void
    @FocusState private var isFocused: Bool

    var body: some View {
        Button(action: action) {
            VStack(alignment: .leading, spacing: 8) {
                Image(systemName: systemImage)
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(Color.accentColor)
                Text(title)
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(.primary)
                Text(description)
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.leading)
                    .textSelection(.enabled)
            }
            .frame(maxWidth: .infinity, minHeight: 88, alignment: .leading)
            .padding(12)
        }
        .buttonStyle(SettingsActionTileButtonStyle(isFocused: isFocused))
        .focusable()
        .focused($isFocused)
    }
}

private struct SettingsActionTileButtonStyle: ButtonStyle {
    let isFocused: Bool

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .background(tileBackground(isPressed: configuration.isPressed))
            .overlay(tileOutline(isPressed: configuration.isPressed))
            .contentShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
    }

    private func tileBackground(isPressed: Bool) -> some View {
        RoundedRectangle(cornerRadius: 10, style: .continuous)
            .fill(
                isPressed
                    ? Color.accentColor.opacity(0.18)
                    : Color(nsColor: .quaternaryLabelColor).opacity(0.15)
            )
    }

    private func tileOutline(isPressed: Bool) -> some View {
        RoundedRectangle(cornerRadius: 10, style: .continuous)
            .stroke(
                isFocused || isPressed ? Color.accentColor : .clear,
                lineWidth: isFocused || isPressed ? 1 : 0
            )
    }
}
