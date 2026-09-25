import AppKit
import CoreGraphics
import SwiftUI

struct PopoverView: View {
    @ObservedObject var viewModel: UsageViewModel

    private static let popoverWidth: CGFloat = 640
    private static let popoverMaxHeight: CGFloat = 620
    private static let estimatedChromeHeight: CGFloat = 294
    private static let estimatedProviderRowHeight: CGFloat = 150

    private let providerColumnSpacing: CGFloat = 10

    static func preferredSize(for providerCount: Int) -> CGSize {
        let normalizedCount = max(providerCount, 1)
        let rows = Int(ceil(Double(normalizedCount) / 2.0))
        let height = estimatedChromeHeight + (CGFloat(rows) * estimatedProviderRowHeight)
        return CGSize(width: popoverWidth, height: min(height, popoverMaxHeight))
    }

    var body: some View {
        let preferredSize = Self.preferredSize(for: viewModel.snapshot.providers.count)

        Group {
            if viewModel.snapshot.isPlaceholder {
                loadingPlaceholder
            } else {
                loadedContent
            }
        }
        .frame(width: preferredSize.width, height: preferredSize.height, alignment: .topLeading)
    }

    // First-load state: no placeholder provider cards, just the header and
    // a single spinner centered in the popover.
    private var loadingPlaceholder: some View {
        VStack {
            HeaderView(snapshot: viewModel.snapshot, isRefreshing: viewModel.isRefreshing)
                .frame(maxWidth: .infinity, alignment: .leading)
            Spacer()
            ProgressView()
                .controlSize(.large)
            Spacer()
        }
        .padding(16)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        .background(Color(nsColor: .windowBackgroundColor))
    }

    private var loadedContent: some View {
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
                    onRefresh: { viewModel.refresh() },
                    onRestartHelper: { viewModel.restartHelper() }
                )
            }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .background(Color(nsColor: .windowBackgroundColor))
        .scrollIndicators(.visible)
    }

    private var providerSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Providers (remaining usage)")
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

            // Eager two-column layout (left column = even indices, matching
            // the old LazyVGrid's row-major fill). LazyVGrid measures its
            // children without a width proposal and doesn't re-measure, so
            // the cards' full-width usage rows reported too little height
            // and spilled past the card background. Provider counts are
            // small, so eager layout costs nothing and sizes correctly.
            let providers = viewModel.snapshot.providers
            // Column widths measured once from every provider's row strings,
            // so each column fits its widest content and the bar is the same
            // width in every row of every card.
            let columnWidths = UsageRowMetrics.columnWidths(for: providers)
            HStack(alignment: .top, spacing: providerColumnSpacing) {
                providerColumn(providers.enumerated().filter { $0.offset.isMultiple(of: 2) }.map(\.element), columnWidths: columnWidths)
                providerColumn(providers.enumerated().filter { !$0.offset.isMultiple(of: 2) }.map(\.element), columnWidths: columnWidths)
            }
        }
    }

    private func providerColumn(_ providers: [ProviderCard], columnWidths: UsageRowMetrics.ColumnWidths) -> some View {
        VStack(alignment: .leading, spacing: providerColumnSpacing) {
            ForEach(providers) { provider in
                ProviderCardView(provider: provider, columnWidths: columnWidths, isRefreshing: viewModel.isRefreshing)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
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
