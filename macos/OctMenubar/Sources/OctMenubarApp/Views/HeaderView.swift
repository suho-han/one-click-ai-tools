import SwiftUI

struct HeaderView: View {
    let snapshot: UsageSnapshot
    @ObservedObject var releaseMonitor: ReleaseUpdateMonitor
    @ObservedObject private var installer: ReleaseUpdateInstaller

    init(snapshot: UsageSnapshot, releaseMonitor: ReleaseUpdateMonitor) {
        self.snapshot = snapshot
        self.releaseMonitor = releaseMonitor
        self.installer = releaseMonitor.installer
    }

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
            installer.requestUpdate(to: update)
        } label: {
            Text("⬇️ update")
                .font(.system(size: 11, weight: .semibold))
                .padding(.horizontal, 8)
                .padding(.vertical, 3)
                .background(Capsule().fill(Color.accentColor.opacity(0.18)))
        }
        .buttonStyle(.plain)
        .disabled(installer.isUpdating)
        .help(installer.isUpdating ? "oct update is running" : "A newer oct release (\(update.tag)) is available — confirm oct update")
        .accessibilityLabel("Update available; confirm oct update")
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
