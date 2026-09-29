import XCTest

@testable import OctMenubarApp

/// The update prompt must only appear for a strictly newer stable release:
/// equal versions, downgrades, prerelease tags sharing a numeric core, and
/// unparseable local "dev" builds all have to stay silent.
final class ReleaseUpdateTests: XCTestCase {
    func testParseTagAcceptsVAndPrerelease() {
        XCTAssertEqual(ReleaseUpdateLogic.parseTag("v0.1.7"), [0, 1, 7])
        XCTAssertEqual(ReleaseUpdateLogic.parseTag("0.1.7"), [0, 1, 7])
        XCTAssertEqual(ReleaseUpdateLogic.parseTag("v0.1.7-beta.1"), [0, 1, 7])
        XCTAssertEqual(ReleaseUpdateLogic.parseTag(" v0.10.2 "), [0, 10, 2])
    }

    func testParseTagRejectsGarbage() {
        XCTAssertNil(ReleaseUpdateLogic.parseTag("dev"))
        XCTAssertNil(ReleaseUpdateLogic.parseTag(""))
        XCTAssertNil(ReleaseUpdateLogic.parseTag("v0.x.3"))
        XCTAssertNil(ReleaseUpdateLogic.parseTag("release-candidate"))
    }

    func testIsNewerComparesComponentWise() {
        XCTAssertTrue(ReleaseUpdateLogic.isNewer(current: [0, 1, 6], remote: [0, 1, 7]))
        XCTAssertTrue(ReleaseUpdateLogic.isNewer(current: [0, 1, 9], remote: [0, 2, 0]))
        XCTAssertFalse(ReleaseUpdateLogic.isNewer(current: [0, 1, 6], remote: [0, 1, 6]))
        XCTAssertFalse(ReleaseUpdateLogic.isNewer(current: [0, 1, 7], remote: [0, 1, 6]))
        XCTAssertFalse(ReleaseUpdateLogic.isNewer(current: [1, 0, 0], remote: [0, 9, 9]))
    }

    func testIsNewerPadsMissingComponentsWithZero() {
        XCTAssertFalse(ReleaseUpdateLogic.isNewer(current: [0, 1], remote: [0, 1, 0]))
        XCTAssertTrue(ReleaseUpdateLogic.isNewer(current: [0, 1], remote: [0, 1, 1]))
    }

    func testUpdateIfNewerPromptsOnlyForNewerRelease() {
        let update = ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.6",
            remoteTag: "v0.1.7",
            htmlURL: "https://github.com/suho-han/one-click-ai-tools/releases/tag/v0.1.7"
        )
        XCTAssertEqual(update?.tag, "v0.1.7")

        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.7", remoteTag: "v0.1.7", htmlURL: "https://example.com"
        ))
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.2.0", remoteTag: "v0.1.7", htmlURL: "https://example.com"
        ))
        // A prerelease sharing the numeric core is not an update prompt.
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.7", remoteTag: "v0.1.7-beta.1", htmlURL: "https://example.com"
        ))
        // Local "dev" builds never prompt.
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "dev", remoteTag: "v0.1.7", htmlURL: "https://example.com"
        ))
    }

    func testUpdateIfNewerRejectsUnusableDownloadURL() {
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.6", remoteTag: "v0.1.7", htmlURL: "not a url"
        ))
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.6", remoteTag: "v0.1.7", htmlURL: "ftp://example.com/v0.1.7"
        ))
    }
}
