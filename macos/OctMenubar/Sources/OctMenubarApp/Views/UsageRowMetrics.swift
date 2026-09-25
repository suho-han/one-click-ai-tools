import AppKit

/// Column widths for the provider card usage rows, measured from the actual
/// row strings instead of hardcoded: each column is exactly as wide as its
/// widest content across every provider card — the smallest width that never
/// truncates — so the bar (whatever is left over) is identical on every row
/// and card.
///
/// Measured with the NSFont SwiftUI resolves for
/// `.system(size: 9, weight: .semibold, design: .rounded)`, which is the
/// font the usage rows render with, keeping the widths in sync with what
/// Text actually draws.
enum UsageRowMetrics {
    static let rowFontSize: CGFloat = 9

    typealias ColumnWidths = (label: CGFloat, value: CGFloat, countdown: CGFloat)

    static func columnWidths(for providers: [ProviderCard]) -> ColumnWidths {
        var labels: [String] = []
        var values: [String] = []
        var countdowns: [String] = []
        for provider in providers {
            for metric in provider.metrics {
                labels.append(metric.label)
                values.append(metric.value)
                if let resetsIn = metric.resetsIn {
                    countdowns.append(resetsIn)
                }
            }
        }
        // +2pt headroom over the measured maximum so sub-pixel differences
        // between NSAttributedString sizing and SwiftUI Text never clip.
        return (
            label: width(of: labels) + 2,
            value: width(of: values) + 2,
            countdown: width(of: countdowns) + 2
        )
    }

    static func width(of strings: [String]) -> CGFloat {
        guard !strings.isEmpty else {
            return 0
        }
        let base = NSFont.systemFont(ofSize: rowFontSize, weight: .semibold)
        let descriptor = base.fontDescriptor.withDesign(.rounded) ?? base.fontDescriptor
        let rounded = NSFont(descriptor: descriptor, size: rowFontSize) ?? base
        let maxWidth = strings.reduce(into: CGFloat(0)) { widest, string in
            let size = NSAttributedString(string: string, attributes: [.font: rounded]).size()
            widest = max(widest, size.width)
        }
        return ceil(maxWidth)
    }
}
