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
                statusPill
            }

            if let note = snapshot.note {
                Text(note)
                    .font(.system(size: 11))
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

}
