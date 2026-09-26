import Foundation
import XCTest
@testable import OctMenubarApp

final class ProviderLogoTests: XCTestCase {
    func testKnownProvidersResolveToDecodedImages() {
        let names = [
            "claude-code", "claude", "claude:personal",
            "codex", "codex:second", "openai",
            "cursor", "cursor-agent",
            "agy", "antigravity", "gemini",
            "commandcode", "command code",
            "copilot", "github-copilot", "github",
            "opencode",
        ]
        for name in names {
            let image = ProviderLogo.image(for: name)
            XCTAssertNotNil(image, "expected a logo for \(name)")
            XCTAssertEqual(image?.size.width ?? 0, 128, "logo for \(name) should decode at its full 128pt width")
        }
    }

    func testProvidersWithoutLogoAssetsReturnNil() {
        let names = [
            "kimi", "zai", "qwen", "deepseek", "openrouter", "minimax", "grok",
            "", "unknown-provider",
        ]
        for name in names {
            XCTAssertNil(ProviderLogo.image(for: name), "did not expect a logo for \(name)")
        }
    }

    func testImageIsCachedAcrossCalls() {
        let first = ProviderLogo.image(for: "codex")
        let second = ProviderLogo.image(for: "codex:personal")
        XCTAssertTrue(first === second, "repeat lookups should reuse the decoded NSImage")
    }

    func testBase64PayloadsStillDecode() {
        // Guards against the embedded payloads being corrupted by source
        // edits: every constant must decode as valid PNG bytes.
        for name in ["claude-code", "commandcode", "codex", "cursor", "gemini", "copilot", "opencode"] {
            let image = ProviderLogo.image(for: name)
            XCTAssertNotNil(image, "embedded logo for \(name) failed to decode")
            XCTAssertGreaterThan(image?.tifFRepresentationDataSize ?? 0, 0)
        }
    }
}

private extension NSImage {
    /// Cheap "has real backing store" probe without importing more frameworks.
    var tifFRepresentationDataSize: Int {
        tiffRepresentation?.count ?? 0
    }
}
