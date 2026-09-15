import SwiftUI

struct HeaderView: View {
    let snapshot: UsageSnapshot
    let isRefreshing: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 4) {
                    Text(snapshot.title)
                        .font(.system(size: 19, weight: .semibold))
                    Text(snapshot.summaryLine)
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(.secondary)
                }
                Spacer()
                if let note = snapshot.note?.trimmingCharacters(in: .whitespacesAndNewlines), !note.isEmpty {
                    sourceInfoButton(note: note)
                }
                statusPill
            }
        }
    }

    private func sourceInfoButton(note: String) -> some View {
        Button(action: {}) {
            Image(systemName: "info.circle")
                .font(.system(size: 13, weight: .semibold))
                .foregroundStyle(.secondary)
                .frame(width: 24, height: 24)
                .contentShape(Circle())
        }
        .buttonStyle(.plain)
        .help(note)
        .accessibilityLabel("Usage data source")
        .accessibilityHint(note)
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

}
