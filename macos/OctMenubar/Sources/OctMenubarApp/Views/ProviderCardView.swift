import SwiftUI

struct ProviderCardView: View {
    let provider: ProviderCard

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .top, spacing: 6) {
                HStack(alignment: .top, spacing: 6) {
                    Circle()
                        .fill(provider.status.tint)
                        .frame(width: 8, height: 8)
                        .padding(.top, 4)

                    VStack(alignment: .leading, spacing: 2) {
                        Text(provider.name)
                            .font(.system(size: 13, weight: .semibold))
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

            planStrip

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
                    Capsule().fill(Color(nsColor: .windowBackgroundColor).opacity(0.8))
                )
            }
        }
    }

    private var planStrip: some View {
        HStack(spacing: 6) {
            Text("PLAN")
                .foregroundStyle(.secondary)
            Text(provider.plan)
                .foregroundStyle(provider.plan == "unknown" ? .secondary : .primary)
            Spacer(minLength: 0)
        }
        .font(.system(size: 9, weight: .semibold, design: .rounded))
        .padding(.horizontal, 7)
        .padding(.vertical, 4)
        .background(
            Capsule().fill(Color(nsColor: .windowBackgroundColor).opacity(0.8))
        )
    }
}
