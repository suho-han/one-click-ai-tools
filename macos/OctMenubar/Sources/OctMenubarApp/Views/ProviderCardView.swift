import SwiftUI

struct ProviderCardView: View {
    let provider: ProviderCard

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .top, spacing: 10) {
                Circle()
                    .fill(provider.status.tint)
                    .frame(width: 10, height: 10)
                    .padding(.top, 4)

                VStack(alignment: .leading, spacing: 4) {
                    Text(provider.name)
                        .font(.system(size: 15, weight: .semibold))
                    Text(provider.metricsLine)
                        .font(.system(size: 12, weight: .medium, design: .rounded))
                        .foregroundStyle(.secondary)
                }

                Spacer()

                Text(provider.status.badgeLabel)
                    .font(.system(size: 11, weight: .bold, design: .rounded))
                    .foregroundStyle(provider.status.tint)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 6)
                    .background(
                        Capsule().fill(provider.status.softBackground)
                    )
            }

            metricStrip

            if let message = provider.message, !message.isEmpty {
                Text(message)
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
                    .lineLimit(3)
            }
        }
        .padding(14)
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
        HStack(spacing: 8) {
            ForEach(Array(provider.metrics.enumerated()), id: \.offset) { _, metric in
                HStack(spacing: 6) {
                    Text(metric.label)
                        .foregroundStyle(.secondary)
                    Text(metric.value)
                        .foregroundStyle(.primary)
                }
                .font(.system(size: 11, weight: .semibold, design: .rounded))
                .padding(.horizontal, 10)
                .padding(.vertical, 6)
                .background(
                    Capsule().fill(Color(nsColor: .windowBackgroundColor).opacity(0.8))
                )
            }
        }
    }
}
