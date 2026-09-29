import SwiftUI

struct HeaderView: View {
    let snapshot: UsageSnapshot
    let isRefreshing: Bool
    @ObservedObject var releaseMonitor: ReleaseUpdateMonitor

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 4) {
                    HStack(alignment: .firstTextBaseline, spacing: 6) {
                        Text(snapshot.title)
                            .font(.system(size: 19, weight: .semibold))
                        Text(Self.versionLabel)
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(.secondary)
                        if let update = releaseMonitor.availableUpdate {
                            updateButton(update)
                        }
                    }
                    Text(snapshot.summaryLine)
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(.secondary)
                }
                Spacer()
                statusPill
            }
        }
    }

    /// The running helper's version, shown small beside the title so the
    /// panel doubles as the "which release am I on" answer.
    private static var versionLabel: String {
        let raw = MenubarBuildVersion.version
        return raw == "dev" || raw.hasPrefix("v") ? raw : "v\(raw)"
    }

    private func updateButton(_ update: ReleaseUpdate) -> some View {
        Button {
            NSWorkspace.shared.open(update.htmlURL)
        } label: {
            Text("⬇️ update")
                .font(.system(size: 11, weight: .semibold))
                .padding(.horizontal, 8)
                .padding(.vertical, 3)
                .background(Capsule().fill(Color.accentColor.opacity(0.18)))
        }
        .buttonStyle(.plain)
        .help("A newer oct release (\(update.tag)) is available — open the download page")
        .accessibilityLabel("Update available; open the release download page")
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
