import AppKit
import SwiftUI

struct FooterActionsView: View {
    let isRefreshing: Bool
    let onRefresh: () -> Void
    let onAction: (OctMenubarAction) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Actions")
                .font(.system(size: 13, weight: .semibold))
                .foregroundStyle(.secondary)

            HStack(spacing: 10) {
                actionButton(title: isRefreshing ? "Refreshing…" : "Refresh now", systemImage: "arrow.clockwise", emphasized: true, disabled: isRefreshing) {
                    onRefresh()
                }
                actionButton(title: "Run alert", systemImage: "bell.badge", emphasized: false) {
                    onAction(.runAlertCheck)
                }
            }

            HStack(spacing: 10) {
                actionButton(title: "Open usage", systemImage: "chart.bar") {
                    onAction(.openUsage)
                }
                actionButton(title: "Open monitor", systemImage: "waveform.path.ecg") {
                    onAction(.openMonitor)
                }
            }

            HStack(spacing: 10) {
                actionButton(title: "Session refresh", systemImage: "key.horizontal") {
                    onAction(.runSessionRefresh)
                }
                actionButton(title: "Quit helper", systemImage: "xmark.circle") {
                    NSApp.terminate(nil)
                }
            }
        }
    }

    private func actionButton(
        title: String,
        systemImage: String,
        emphasized: Bool = false,
        disabled: Bool = false,
        action: @escaping () -> Void
    ) -> some View {
        Button(action: action) {
            Label(title, systemImage: systemImage)
                .font(.system(size: 12, weight: .semibold))
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 12)
                .padding(.vertical, 10)
                .background(
                    RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .fill(emphasized ? Color.accentColor.opacity(0.15) : Color(nsColor: .controlBackgroundColor))
                )
        }
        .buttonStyle(.plain)
        .disabled(disabled)
        .opacity(disabled ? 0.6 : 1)
    }
}
