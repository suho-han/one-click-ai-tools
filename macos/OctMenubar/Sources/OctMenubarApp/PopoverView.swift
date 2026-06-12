import SwiftUI

struct PopoverView: View {
    @ObservedObject var viewModel: UsageViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            header
            Divider()
            providerSection
            Divider()
            footerActions
        }
        .padding(16)
        .frame(width: 360, height: 520, alignment: .topLeading)
        .background(Color(nsColor: .windowBackgroundColor))
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Usage Overview")
                .font(.headline)
            Text(viewModel.snapshot.summaryLine)
                .font(.subheadline)
                .foregroundStyle(.secondary)
            Text("Last refresh: \(viewModel.snapshot.lastRefreshLabel)")
                .font(.caption)
                .foregroundStyle(.secondary)
            Text("Next refresh: \(viewModel.snapshot.nextRefreshLabel)")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }

    private var providerSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Providers")
                .font(.subheadline.weight(.semibold))
            ForEach(viewModel.snapshot.providers) { provider in
                VStack(alignment: .leading, spacing: 4) {
                    HStack {
                        Text(provider.name)
                            .font(.body.weight(.medium))
                        Spacer()
                        Text(provider.status.uppercased())
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(provider.statusColor)
                    }
                    Text(provider.metricsLine)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    if let message = provider.message, !message.isEmpty {
                        Text(message)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .lineLimit(2)
                    }
                }
                .padding(12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(
                    RoundedRectangle(cornerRadius: 12)
                        .fill(Color(nsColor: .controlBackgroundColor))
                )
            }
            Spacer(minLength: 0)
        }
    }

    private var footerActions: some View {
        VStack(alignment: .leading, spacing: 10) {
            Button("Refresh now") {
                viewModel.refresh()
            }
            Button("Open Usage") {
                viewModel.runAction(.openUsage)
            }
            Button("Open Monitor") {
                viewModel.runAction(.openMonitor)
            }
            Button("Run Session Refresh") {
                viewModel.runAction(.runSessionRefresh)
            }
            Button("Run Alert Check") {
                viewModel.runAction(.runAlertCheck)
            }
        }
        .buttonStyle(.plain)
    }
}
