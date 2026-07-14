import SwiftUI

enum SettingsFeedbackKind {
    case informational
    case success
    case warning
    case error

    var systemImage: String {
        switch self {
        case .informational:
            return "info.circle"
        case .success:
            return "checkmark.circle"
        case .warning:
            return "exclamationmark.triangle"
        case .error:
            return "xmark.octagon"
        }
    }

    var color: Color {
        switch self {
        case .informational:
            return .accentColor
        case .success:
            return Color(nsColor: .systemGreen)
        case .warning:
            return Color(nsColor: .systemOrange)
        case .error:
            return Color(nsColor: .systemRed)
        }
    }
}

struct SettingsFeedback {
    let message: String
    let kind: SettingsFeedbackKind

    static func informational(_ message: String) -> Self {
        Self(message: message, kind: .informational)
    }

    static func success(_ message: String) -> Self {
        Self(message: message, kind: .success)
    }

    static func warning(_ message: String) -> Self {
        Self(message: message, kind: .warning)
    }

    static func error(_ message: String) -> Self {
        Self(message: message, kind: .error)
    }
}

struct SettingsTabSelector: View {
    @Binding var selection: SettingsTab

    var body: some View {
        HStack(spacing: 8) {
            ForEach(SettingsTab.allCases) { tab in
                SettingsTabButton(tab: tab, selection: $selection)
            }
        }
        .padding(.bottom, 4)
    }
}

private struct SettingsTabButton: View {
    let tab: SettingsTab
    @Binding var selection: SettingsTab

    var body: some View {
        tabButton
        .buttonStyle(SettingsTabButtonStyle(isSelected: selection == tab))
        .accessibilityValue(selection == tab ? "Selected" : "Not selected")
    }

    private var tabButton: some View {
        Button {
            selection = tab
        } label: {
            HStack(spacing: 6) {
                Image(systemName: tab.systemImage)
                Text(tab.title)
            }
            .frame(maxWidth: .infinity)
        }
    }
}

private struct SettingsTabButtonStyle: ButtonStyle {
    let isSelected: Bool

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .padding(.vertical, 7)
            .background(tabBackground(isPressed: configuration.isPressed))
            .overlay(tabOutline(isPressed: configuration.isPressed))
            .contentShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
    }

    private func tabBackground(isPressed: Bool) -> some View {
        RoundedRectangle(cornerRadius: 10, style: .continuous)
            .fill(
                isSelected || isPressed
                    ? Color.accentColor.opacity(0.24)
                    : Color(nsColor: .quaternaryLabelColor).opacity(0.15)
            )
    }

    private func tabOutline(isPressed: Bool) -> some View {
        RoundedRectangle(cornerRadius: 10, style: .continuous)
            .stroke(
                isSelected || isPressed ? Color.accentColor : .clear,
                lineWidth: isSelected || isPressed ? 1 : 0
            )
    }
}

struct SettingsSectionCard<Content: View>: View {
    let title: String
    let systemImage: String
    let description: String?
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top, spacing: 8) {
                Image(systemName: systemImage)
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(Color.accentColor)
                    .frame(width: 16)

                VStack(alignment: .leading, spacing: 4) {
                    Text(title)
                        .font(.system(size: 13, weight: .semibold))
                    if let description {
                        Text(description)
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                    }
                }
            }

            content()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .fill(Color(nsColor: .controlBackgroundColor))
        )
    }
}

struct SettingsFeedbackMessage: View {
    let feedback: SettingsFeedback
    var compact = false

    var body: some View {
        HStack(spacing: 6) {
            Image(systemName: feedback.kind.systemImage)
            Text(feedback.message)
                .textSelection(.enabled)
        }
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(feedback.kind.color)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 12)
            .padding(.vertical, compact ? 6 : 8)
            .background(
                RoundedRectangle(cornerRadius: 10, style: .continuous)
                    .fill(feedback.kind.color.opacity(0.12))
            )
    }
}
