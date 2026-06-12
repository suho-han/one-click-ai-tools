import SwiftUI

struct PopoverView: View {
    @ObservedObject var viewModel: UsageViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HeaderView(snapshot: viewModel.snapshot, isRefreshing: viewModel.isRefreshing)
            Divider()
            ScrollView(showsIndicators: false) {
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

                    ForEach(viewModel.snapshot.providers) { provider in
                        ProviderCardView(provider: provider)
                    }
                }
            }
            Divider()
            FooterActionsView(
                isRefreshing: viewModel.isRefreshing,
                onRefresh: { viewModel.refresh() },
                onAction: { viewModel.runAction($0) }
            )
        }
        .padding(16)
        .frame(width: 388, height: 560, alignment: .topLeading)
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
}
