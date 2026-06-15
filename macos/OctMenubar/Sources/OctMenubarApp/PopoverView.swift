import AppKit
import CoreGraphics
import SwiftUI

struct PopoverView: View {
    @ObservedObject var viewModel: UsageViewModel

    private static let popoverWidth: CGFloat = 640
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
        return CGSize(width: popoverWidth, height: height)
    }

    var body: some View {
        let preferredSize = Self.preferredSize(for: viewModel.snapshot.providers.count)

        VStack(alignment: .leading, spacing: 16) {
            HeaderView(snapshot: viewModel.snapshot, isRefreshing: viewModel.isRefreshing)
            Divider()
            providerSection
            Divider()
            FooterActionsView(
                isRefreshing: viewModel.isRefreshing,
                onRefresh: { viewModel.refresh() },
                onOpenSettings: {
                    NSApp.activate(ignoringOtherApps: true)
                    NSApp.sendAction(Selector(("showSettingsWindow:")), to: nil, from: nil)
                }
            )
        }
        .padding(16)
        .frame(width: preferredSize.width, height: preferredSize.height, alignment: .topLeading)
        .background(
            LinearGradient(
                colors: [
                    Color(nsColor: .windowBackgroundColor),
                    Color(nsColor: .underPageBackgroundColor),
                ],
                startPoint: .top,
                endPoint: .bottom
            )
        )
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
}
