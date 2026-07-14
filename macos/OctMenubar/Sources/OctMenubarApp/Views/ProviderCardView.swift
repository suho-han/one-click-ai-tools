import SwiftUI

struct ProviderCardView: View {
    let provider: ProviderCard
    @AppStorage(MenubarPreferences.useProviderAccentColorsKey) private var useProviderAccentColors = true

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .top, spacing: 6) {
                HStack(alignment: .top, spacing: 6) {
                    Circle()
                        .fill(provider.status.tint)
                        .frame(width: 8, height: 8)
                        .padding(.top, 4)

                    HStack(alignment: .firstTextBaseline, spacing: 4) {
                        Text(provider.name)
                            .font(.system(size: 13, weight: .semibold))
                            .foregroundStyle(providerAccent)
                            .lineLimit(1)

                        Text("· \(provider.plan)")
                            .font(.system(size: 10, weight: .semibold, design: .rounded))
                            .foregroundStyle(provider.plan == "unknown" ? .secondary : .primary)
                            .lineLimit(1)
                    }
                }

                Spacer(minLength: 6)

                Text(provider.status.badgeLabel)
                    .font(.system(size: 10, weight: .bold, design: .rounded))
                    .foregroundStyle(provider.status.tint)
                    .padding(.horizontal, 7)
                    .padding(.vertical, 4)
                    .background(
                        Capsule().fill(provider.status.softBackground)
                    )
            }

            metricStrip

            if let message = provider.message, !message.isEmpty {
                Text(message)
                    .font(.system(size: 10))
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
        }
        .padding(10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(Color(nsColor: .controlBackgroundColor))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .stroke(provider.status.softBackground, lineWidth: 1)
        )
    }

    private var metricStrip: some View {
        HStack(spacing: 4) {
            ForEach(Array(provider.metrics.enumerated()), id: \.offset) { _, metric in
                HStack(spacing: 4) {
                    Text(metric.label)
                        .foregroundStyle(.secondary)
                    Text(metric.value)
                        .foregroundStyle(.primary)
                }
                .font(.system(size: 9, weight: .semibold, design: .rounded))
                .padding(.horizontal, 7)
                .padding(.vertical, 4)
                .background(
                    Capsule().fill(accentCapsuleBackground)
                )
            }
        }
    }

    private var providerAccent: Color {
        provider.accentColor(useProviderAccentColors: useProviderAccentColors)
    }

    private var accentCapsuleBackground: Color {
        useProviderAccentColors
            ? providerAccent.opacity(0.14)
            : Color(nsColor: .windowBackgroundColor).opacity(0.8)
    }
}
