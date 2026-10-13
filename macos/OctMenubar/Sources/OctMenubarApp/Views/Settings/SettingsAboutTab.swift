import SwiftUI

struct SettingsAboutTab: View {
    @ObservedObject var releaseMonitor: ReleaseUpdateMonitor
    @ObservedObject var feedbackStore: FeedbackStore

    private static let repositoryURL = URL(string: "https://github.com/\(ReleaseUpdateMonitor.repo)")!
    private static let sponsorURL = URL(string: "https://github.com/sponsors/suho-han")!

    var body: some View {
        SettingsSectionCard(
            title: "one-click-tools",
            systemImage: "info.circle",
            description: "AI CLI usage, updates, and alerts from your menu bar."
        ) {
            LabeledContent("App version") {
                Text(Self.versionLabel)
                    .textSelection(.enabled)
            }
        }

        SettingsSectionCard(
            title: "Updates",
            systemImage: "arrow.down.circle",
            description: "Control release notifications for this app."
        ) {
            Toggle("Automatically check for updates", isOn: Binding(
                get: { releaseMonitor.automaticallyChecksForUpdates },
                set: { releaseMonitor.setAutomaticallyChecksForUpdates($0) }
            ))
            Text("Checks GitHub Releases every five minutes. Choose Yes in the update prompt to run oct update; updates are never installed automatically.")
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
        }

        SettingsSectionCard(
            title: "Project",
            systemImage: "link",
            description: "Source code and ways to support development."
        ) {
            Link(destination: Self.repositoryURL) {
                Label("GitHub repository", systemImage: "chevron.left.forwardslash.chevron.right")
            }
            Link(destination: Self.sponsorURL) {
                Label("Sponsor on GitHub", systemImage: "heart")
            }
            Divider()
            Text("Feedback")
                .font(.headline)
            Text("Only the text you submit is sent and retained for 90 days. Usage analytics settings do not affect feedback. Do not include passwords, API keys, or personal information.")
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
            TextEditor(text: $feedbackStore.message)
                .font(.body)
                .frame(minHeight: 100)
                .overlay(RoundedRectangle(cornerRadius: 4).stroke(.separator))
                .accessibilityLabel("Feedback message")
                .disabled(feedbackStore.isSubmitting)
            HStack {
                Text("\(feedbackStore.messageByteCount) / 4000 bytes")
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
                Spacer()
                Button {
                    Task { await feedbackStore.submit() }
                } label: {
                    HStack {
                        if feedbackStore.isSubmitting {
                            ProgressView().controlSize(.small)
                        }
                        Text(feedbackStore.isSubmitting ? "Sending…" : "Send feedback")
                    }
                }
                .disabled(!feedbackStore.canSubmit)
            }
            if !feedbackStore.isAvailable {
                SettingsFeedbackMessage(
                    feedback: .informational("Feedback is unavailable in this build."),
                    compact: true
                )
            } else if feedbackStore.messageByteCount > 4000 {
                SettingsFeedbackMessage(
                    feedback: .warning("Feedback must be 4000 UTF-8 bytes or fewer."),
                    compact: true
                )
            }
            if let status = feedbackStore.status {
                SettingsFeedbackMessage(feedback: status, compact: true)
            }
        }
    }

    private static var versionLabel: String {
        let raw = MenubarBuildVersion.version
        return raw == "dev" || raw.hasPrefix("v") ? raw : "v\(raw)"
    }
}
