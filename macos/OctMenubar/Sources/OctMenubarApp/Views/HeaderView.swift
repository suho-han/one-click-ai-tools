import SwiftUI

struct HeaderView: View {
    let snapshot: UsageSnapshot
    let isRefreshing: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 4) {
                    Text(snapshot.title)
                        .font(.system(size: 20, weight: .semibold))
                    Text(snapshot.summaryLine)
                        .font(.system(size: 13, weight: .medium))
                        .foregroundStyle(.secondary)
                }
                Spacer()
                statusPill
            }

            HStack(spacing: 10) {
                metaCard(label: "Last refresh", value: snapshot.lastRefreshLabel)
                metaCard(label: "Next refresh", value: snapshot.nextRefreshLabel)
            }

            HStack {
                Label(snapshot.autoRefreshLabel, systemImage: isRefreshing ? "arrow.triangle.2.circlepath.circle.fill" : "clock")
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(isRefreshing ? Color.accentColor : .secondary)
                Spacer()
            }

            if let note = snapshot.note {
                Text(note)
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
        }
    }

    private var statusPill: some View {
        Text(snapshot.statusItemTitle.replacingOccurrences(of: "oct", with: "").trimmingCharacters(in: .whitespaces))
            .font(.system(size: 11, weight: .bold, design: .rounded))
            .foregroundStyle(.secondary)
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(
                Capsule().fill(Color(nsColor: .quaternaryLabelColor).opacity(0.15))
            )
            .opacity(snapshot.statusItemTitle == "oct" ? 0 : 1)
    }

    private func metaCard(label: String, value: String) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(.secondary)
            Text(value)
                .font(.system(size: 14, weight: .semibold, design: .rounded))
                .foregroundStyle(.primary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .fill(Color(nsColor: .controlBackgroundColor))
        )
    }
}
