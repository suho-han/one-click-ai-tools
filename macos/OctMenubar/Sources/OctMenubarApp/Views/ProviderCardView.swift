import SwiftUI

struct ProviderCardView: View {
    let provider: ProviderCard
    /// Column widths measured from every provider's row strings (see
    /// UsageRowMetrics) — wide enough that no text ever truncates, and
    /// identical across cards so the bar's width matches everywhere.
    let columnWidths: UsageRowMetrics.ColumnWidths
    /// True while the refresh that will replace this card's data is still in
    /// flight — drives the quota bars' pulsing shimmer.
    var isRefreshing: Bool = false
    @AppStorage(MenubarPreferences.useProviderAccentColorsKey) private var useProviderAccentColors = true
    @State private var showsMessage = false
    @State private var isBarPulsing = false
    /// Measured card width — caps the hover bubble so it never extends past
    /// the card (and therefore the popover) edges.
    @State private var cardWidth: CGFloat = 0

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .top, spacing: 6) {
                HStack(alignment: .top, spacing: 6) {
                    Circle()
                        .fill(provider.status.tint)
                        .frame(width: 8, height: 8)
                        .padding(.top, 4)

                    HStack(alignment: .firstTextBaseline, spacing: 4) {
                        Text(provider.name)
                            .font(.system(size: 13, weight: .semibold))
                            .foregroundStyle(providerAccent)
                            .lineLimit(1)

                        if showsPlan {
                            Text("· \(provider.plan)")
                                .font(.system(size: 10, weight: .semibold, design: .rounded))
                                .foregroundStyle(.primary)
                                .lineLimit(1)
                        }
                    }
                }

                Spacer(minLength: 6)

                statusBadge
            }
            // Lift the whole title row (and the hover bubble anchored to the
            // badge) above the metric strip below: later siblings paint over
            // earlier ones, and without this zIndex the bubble's lower half
            // would be hidden behind the metric rows.
            .zIndex(showsMessage ? 1 : 0)

            if !provider.metrics.isEmpty {
                metricStrip
            }
        }
        .padding(10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(Color(nsColor: .controlBackgroundColor))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .stroke(provider.status.softBackground, lineWidth: 1)
        )
        .background(
            GeometryReader { geo in
                Color.clear
                    .onAppear { cardWidth = geo.size.width }
                    .onChange(of: geo.size.width) { _, width in cardWidth = width }
            }
        )
        .onAppear {
            setBarPulsing(isRefreshing)
        }
        .onChange(of: isRefreshing) { _, refreshing in
            setBarPulsing(refreshing)
        }
    }

    /// While a refresh is in flight the bar fill pulses ("깜박깜박"); when it
    /// lands the pulse settles back to full opacity so the width change to
    /// the new value reads clearly. The repeatForever transaction is bound
    /// to the pulse-on change only, so the pulse-off change runs a plain
    /// ease-out that stops at opacity 1 instead of oscillating forever.
    private func setBarPulsing(_ active: Bool) {
        if active {
            withAnimation(.easeInOut(duration: 0.55).repeatForever(autoreverses: true)) {
                isBarPulsing = true
            }
        } else {
            withAnimation(.easeInOut(duration: 0.25)) {
                isBarPulsing = false
            }
        }
    }

    private var statusBadge: some View {
        Text(provider.status.badgeLabel)
            .font(.system(size: 10, weight: .bold, design: .rounded))
            .foregroundStyle(provider.status.tint)
            .padding(.horizontal, 7)
            .padding(.vertical, 4)
            .background(
                Capsule().fill(provider.status.softBackground)
            )
            .overlay(alignment: .bottomTrailing) {
                if showsMessage, let message = provider.message, !message.isEmpty {
                    messageBubble(message)
                        .offset(y: 28)
                        .transition(.opacity)
                        .allowsHitTesting(false)
                }
            }
            .onHover { hovering in
                guard hasMessage else { return }
                showsMessage = hovering
            }
            .animation(.easeOut(duration: 0.12), value: showsMessage)
            .accessibilityHint(provider.message ?? "")
    }

    /// Fully opaque tooltip anchored to the badge's trailing edge so it extends
    /// left across the card — the popover clips anything past its right
    /// edge, where a centered bubble would end up for cards in the right
    /// grid column. Text is leading-aligned so the message starts at the
    /// front of the block, and the fixed gray fill is deliberately different
    /// from both the card and popover backgrounds: same-color fills read as
    /// translucent wherever the bubble hangs over bare popover background.
    private func messageBubble(_ message: String) -> some View {
        Text(message)
            .font(.system(size: 13))
            .foregroundStyle(Color.white)
            .multilineTextAlignment(.leading)
            .padding(.horizontal, 12)
            .padding(.vertical, 9)
            .frame(width: bubbleWidth, alignment: .leading)
            .fixedSize(horizontal: false, vertical: true)
            .background(
                RoundedRectangle(cornerRadius: 9, style: .continuous)
                    .fill(Color(white: 0.30, opacity: 1.0))
                    .shadow(color: .black.opacity(0.4), radius: 6, y: 3)
            )
            .overlay(
                RoundedRectangle(cornerRadius: 9, style: .continuous)
                    .stroke(Color.white.opacity(0.18), lineWidth: 1)
            )
            .zIndex(10)
    }

    /// Bubble width clamped to the card: the bubble is trailing-anchored to
    /// the badge, which sits at the card's trailing inner edge (10pt card
    /// padding), so width <= cardWidth - 20 keeps both bubble edges inside
    /// the card and thus inside the popover, however narrow the column is.
    /// 340 is the preferred max; 200 keeps very short messages from wrapping
    /// into a sliver if a card ever measures tiny.
    private var bubbleWidth: CGFloat {
        let preferred: CGFloat = 340
        let maximum = cardWidth - 20
        guard maximum > 0 else {
            return preferred
        }
        return min(preferred, max(200, maximum))
    }

    private var hasMessage: Bool {
        guard let message = provider.message else { return false }
        return !message.isEmpty
    }

    private var metricStrip: some View {
        VStack(spacing: 4) {
            ForEach(Array(provider.metrics.enumerated()), id: \.offset) { _, metric in
                metricRow(metric)
            }
        }
    }

    /// One usage row, stretched to the card's full width and split into
    /// measured fixed-width blocks — label | bar | percentage | countdown —
    /// so the bar's width is identical on every row and the text blocks
    /// never resize when the fill or the countdown changes. The countdown
    /// block reserves its width even when empty so rows without a reset
    /// time keep the same bar width. Non-percent buckets (request counts)
    /// have no bar: the value flexes after the label instead.
    private func metricRow(_ metric: UsageMetric) -> some View {
        HStack(spacing: 6) {
            Text(metric.label)
                .lineLimit(1)
                .frame(width: columnWidths.label, alignment: .leading)
            if let percent = metric.percent {
                quotaBar(fillFraction: percent / 100)
                Text(metric.value)
                    .lineLimit(1)
                    .frame(width: columnWidths.value, alignment: .leading)
            } else {
                Spacer(minLength: 6)
                Text(metric.value)
                    .lineLimit(1)
                    .foregroundStyle(.primary)
            }
            Text(metric.resetsIn ?? "")
                .foregroundStyle(.secondary)
                .lineLimit(1)
                .frame(width: columnWidths.countdown, alignment: .trailing)
        }
        .font(.system(size: 9, weight: .semibold, design: .rounded))
        .padding(.horizontal, 8)
        .padding(.vertical, 5)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            Capsule().fill(accentCapsuleBackground)
        )
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(metricAccessibilityLabel(metric))
    }

    /// Full-width remaining-quota bar: the fill shrinks as quota is
    /// consumed, mirroring the "5-hour remaining" bars in the web usage
    /// page. GeometryReader is what makes the fill proportional — the bar
    /// itself absorbs whatever horizontal space the label, percentage, and
    /// countdown leave over.
    private func quotaBar(fillFraction: Double) -> some View {
        let clamped = min(max(fillFraction, 0), 1)
        return GeometryReader { geo in
            ZStack(alignment: .leading) {
                Capsule()
                    .fill(Color.primary.opacity(0.10))
                Capsule()
                    .fill(providerAccent)
                    .frame(width: geo.size.width * clamped)
                    .opacity(isBarPulsing ? 0.35 : 1)
                    .animation(.easeInOut(duration: 0.45), value: clamped)
            }
        }
        .frame(height: 4)
        .accessibilityHidden(true)
    }

    private func metricAccessibilityLabel(_ metric: UsageMetric) -> String {
        var parts = [metric.label, metric.value]
        if let resetsIn = metric.resetsIn {
            parts.append("resets in \(resetsIn)")
        }
        return parts.joined(separator: ", ")
    }

    private var showsPlan: Bool {
        let plan = provider.plan.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        return !plan.isEmpty && plan != "unknown" && plan != "n/a"
    }

    private var providerAccent: Color {
        provider.accentColor(useProviderAccentColors: useProviderAccentColors)
    }

    private var accentCapsuleBackground: Color {
        useProviderAccentColors
            ? providerAccent.opacity(0.14)
            : Color(nsColor: .windowBackgroundColor).opacity(0.8)
    }
}
