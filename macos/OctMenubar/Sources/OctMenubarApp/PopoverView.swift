import AppKit
import CoreGraphics
import SwiftUI

struct PopoverView: View {
    @ObservedObject var viewModel: UsageViewModel

    private static let popoverWidth: CGFloat = 640
    private static let popoverMaxHeight: CGFloat = 620
    private static let estimatedChromeHeight: CGFloat = 294
    private static let estimatedProviderRowHeight: CGFloat = 122

    private let providerColumns = [
        GridItem(.flexible(minimum: 0, maximum: .infinity), spacing: 10, alignment: .top),
        GridItem(.flexible(minimum: 0, maximum: .infinity), spacing: 10, alignment: .top),
    ]

    static func preferredSize(for providerCount: Int) -> CGSize {
        let normalizedCount = max(providerCount, 1)
        let rows = Int(ceil(Double(normalizedCount) / 2.0))
        let height = estimatedChromeHeight + (CGFloat(rows) * estimatedProviderRowHeight)
        return CGSize(width: popoverWidth, height: min(height, popoverMaxHeight))
    }

    var body: some View {
        let preferredSize = Self.preferredSize(for: viewModel.snapshot.providers.count)

        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 16) {
                HeaderView(snapshot: viewModel.snapshot, isRefreshing: viewModel.isRefreshing)
                Divider()
                providerSection
                Divider()
                refreshMetadataSection
                Divider()
                FooterActionsView(
                    isRefreshing: viewModel.isRefreshing,
                    onRefresh: { viewModel.refresh() }
                )
            }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .frame(width: preferredSize.width, height: preferredSize.height, alignment: .topLeading)
        .background(Color(nsColor: .windowBackgroundColor))
        .scrollIndicators(.visible)
    }

    private var providerSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Providers")
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(.secondary)
                Spacer()
                Text("\(viewModel.snapshot.providers.count)")
                    .font(.system(size: 12, weight: .semibold, design: .rounded))
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .background(
                        Capsule().fill(Color(nsColor: .quaternaryLabelColor).opacity(0.15))
                    )
            }

            LazyVGrid(columns: providerColumns, alignment: .leading, spacing: 10) {
                ForEach(viewModel.snapshot.providers) { provider in
                    ProviderCardView(provider: provider)
                }
            }
        }
    }

    private var refreshMetadataSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                metadataCard(label: "Last refresh", value: viewModel.snapshot.lastRefreshLabel)
                metadataCard(label: "Next refresh", value: viewModel.snapshot.nextRefreshLabel)
            }

            HStack {
                Label(
                    viewModel.snapshot.autoRefreshLabel,
                    systemImage: viewModel.isRefreshing ? "arrow.triangle.2.circlepath.circle.fill" : "clock"
                )
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(viewModel.isRefreshing ? Color.accentColor : .secondary)
                Spacer()
            }
        }
    }

    private func metadataCard(label: String, value: String) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            Text(label)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(.secondary)
            Text(value)
                .font(.system(size: 13, weight: .semibold, design: .rounded))
                .foregroundStyle(.primary)
            Spacer(minLength: 0)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 10)
        .padding(.vertical, 8)
        .background(
            RoundedRectangle(cornerRadius: 8, style: .continuous)
                .fill(Color(nsColor: .controlBackgroundColor))
        )
    }
}
