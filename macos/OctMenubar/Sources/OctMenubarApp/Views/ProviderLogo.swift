import AppKit

/// Provider service logos shown where the status dot used to sit on each
/// provider card. Each PNG (128x128, alpha) is embedded as base64 in source
/// rather than declared as SwiftPM resources: the release tarball ships the
/// bare OctMenubarApp executable with no sidecar resource bundle (see the
/// Swift menubar helper step in .github/workflows/release.yml), so a
/// Bundle.module lookup would silently miss in production. Source mirrors the
/// source assets at internal/ui/assets/icons.
enum ProviderLogo {
    /// Decoded logo for a provider display name, or nil when the provider has
    /// no logo asset (callers fall back to the status dot). Matching follows
    /// the same lowercase-substring rules as ProviderCard.compactLabel so
    /// account rows like "codex:personal" resolve like their parent provider.
    static func image(for providerName: String) -> NSImage? {
        guard let asset = asset(for: providerName) else {
            return nil
        }
        return image(for: asset)
    }

    private static func asset(for providerName: String) -> LogoAsset? {
        let name = providerName.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        if name.contains("claude") {
            return .claudecode
        }
        if name.contains("commandcode") || name.contains("command code") {
            return .commandcode
        }
        if name.contains("codex") || name.contains("openai") {
            return .codex
        }
        if name.contains("antigravity") || name.contains("gemini") || name == "agy" {
            return .geminicli
        }
        if name.contains("cursor") {
            return .cursor
        }
        if name.contains("copilot") || name.contains("github") {
            return .githubcopilot
        }
        if name.contains("opencode") {
            return .opencode
        }
        return nil
    }

    private enum LogoAsset: String, CaseIterable {
        case claudecode, commandcode, codex, cursor, geminicli, githubcopilot, opencode
    }

    /// Images decode once into an immutable map: card bodies re-evaluate on
    /// every refresh, and NSImage(data:) would re-decode the PNG each call
    /// otherwise. The payloads together are ~18 KB, so eager decoding is
    /// free — and a `let` keeps the cache concurrency-safe under Swift 6.
    /// Source PNGs are black-with-alpha (matching the existing icon assets),
    /// so they're marked template: macOS re-tints them per appearance, keeping
    /// the logos visible on the card in dark mode.
    private static let images: [LogoAsset: NSImage] = {
        var map: [LogoAsset: NSImage] = [:]
        for asset in LogoAsset.allCases {
            let data = Data(base64Encoded: pngData(for: asset)) ?? Data()
            let image = NSImage(data: data) ?? NSImage()
            image.isTemplate = true
            map[asset] = image
        }
        return map
    }()

    private static func image(for asset: LogoAsset) -> NSImage {
        images[asset] ?? NSImage()
    }

    private static func pngData(for asset: LogoAsset) -> String {
        switch asset {
        case .claudecode: return claudecodePNG
        case .commandcode: return commandcodePNG
        case .codex: return codexPNG
        case .cursor: return cursorPNG
        case .geminicli: return geminicliPNG
        case .githubcopilot: return githubcopilotPNG
        case .opencode: return opencodePNG
        }
    }

    private static let claudecodePNG =
        "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" +
        "a/7b/gfLsWJNYUTRYgAAAABJRU5ErkJggg=="

    private static let codexPNG =
        "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" +
        "rAAAAABJRU5ErkJggg=="

    private static let cursorPNG =
        "iVBORw0KGgoAAAANSUhEUgAAAIAAAACACAYAAADDPmHLAAAC10lEQVR4nO3cMW7bQBBG4SWQQl2czi7lVFET3SD2jXKU3MjK" +
        "DeRGqWKVchencxGAmYU6ewANtVjv0v/7AGK38EAA/ahCBDkkSCMAcQQgjgDEEYA4AhBHAOIIQBwBiCMAcQQgjgDEEYA4AhBH" +
        "AOIIQFxxALuHx9GWGRh/rq6vblLA7uGwsVPzLc3A6vpysOVsRcMZAbRFAGEE4CkazgigLQIIIwBP0XBGAG0RQBgBeIqGMwJo" +
        "iwDCCMBTNJwRQFtzCuCv/RO2trYxDtvV58vvtjtp9/vxRxrGtW0bGfJnf7TjpBkFEL8C1U35BiKAd4gAxBGAOAIQRwDiCEAc" +
        "AYgjAHEEII4AxBGAOAIQRwDiCEAcATj2+z8Xz+nfV9t2b5E+3C+Xn55sexYCcPzaH27GcbizbfeGYbz9srzapDMRgIMAfATQ" +
        "IQJwEUAUATgIwEcAHSIAFwFEEYCDAHwE0CECcBFAFAE4CMBHAB0iABcBRBGAgwB8BNAhsQAOmxQx4fl8j1QAE95PUHJRZYMd" +
        "s6AUwFsigAoIoAICqIMAKiCACgigDgKogAAqIIA6CKACAqiAAOoggAqkArB7AXe2nDame34Kjjn+FJxCj8HZvYBbW8422FHE" +
        "AhhtCeBmUNTx/spsbgYRwEsE4CKAKAJwEICPADpEAC4CiCIABwH4CKBDBOAigCgCcBCAjwA6RAAuAogiAAcB+AigQwTgIoAo" +
        "AnAcXxX7vLZt9xZpseVVsa+UBaCEAMQRgDgCEEcA4ghAHAGIIwBxBCCOAMQRgDgCEEcA4ghAHAGIe5cB2B892YdtbdvGhPcT" +
        "THk+vwY7V2s7Vxe2PWk2AbQX/waacgW2RgBhBOApGs4IoC0CCCMAT9FwRgBtEUAYAXiKhjMCaIsAwgjAUzScEUBbBBBGAJ6i" +
        "YcwfAYgjAHEEII4AxBGAOAIQRwDiCEAcAYgjAHEEII4AxBGAOAIQRwDi/gMYvsO9wWcFOAAAAABJRU5ErkJggg=="

    private static let geminicliPNG =
        "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" +
        "UCMAAAAASUVORK5CYII="

    private static let githubcopilotPNG =
        "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"

    private static let opencodePNG =
        "iVBORw0KGgoAAAANSUhEUgAAAIAAAACACAYAAADDPmHLAAACaElEQVR4nO3dMU4bURhF4Rl5DxEpQ6QUbsIOQnaUnYQdATuA" +
        "xkhI4BIEDfTA4wELYJDmavg555PQvMrC16eyLc84CM0A4AwAzgDgDADOAOAMAM4A4AwAzgDgDADOAOAMAM4A4AwAzgDgDADO" +
        "AOAMAM4A4AwAzgDgDADOAOAMAM4A4AwAzgDgDADOAOAMAM4A4AwAzgDgDADOAOAMAC4SwObi+qA/8u9+/PTWuzt/++Vd59ub" +
        "vYf29L8fl9GG0/XPnX/9NKux/81uc3l11B/6z1BAD2Dsl3edba/2WxsP+3Eh7Xi9+31/mNmkJ/9RBpBgABEGEGAACQYQYQAB" +
        "BpBgABEGEGAACQYQYQABBpDwRQOY+gIoIzK+AdQRGd8A6oiMbwB1RMY3gDoi4xtAHZHxDaCOyPgGUEdkfAOoIzK+AdQRGb9S" +
        "AGfb20n/59JWQ7v/9ePbST/OKjJ+pQA2l9etXwrws4AIAwgwgAQDiDCAAANIMIAIAwgwgAQDiDCAAANIMIAIAwioFMDLt32H" +
        "AlbD6s63gjW7yPgGUEdkfAOoIzK+AdQRGd8A6oiMbwB1RMY3gDoi4xtAHZHxDaCOyPgGUEdk/I8EsLSpAb68ZewPRExkAAkG" +
        "EGEAAQaQYAARBhBgAAkGEGEAAQaQYAARBhBgAAmVAni9aVTb68dPb+qobzeNejzox2W08aTMTaNUhwHAGQCcAcAZAJwBwBkA" +
        "nAHAGQCcAcAZAJwBwBkAnAHAGQCcAcAZAJwBwBkAnAHAGQCcAcAZAJwBwBkAnAHAGQCcAcAZAJwBwBkAnAHAGQCcAcAZAJwB" +
        "wBkA3DNZdNKQZKxebAAAAABJRU5ErkJggg=="

    private static let commandcodePNG =
        "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" +
        "RK5CYII="
}
