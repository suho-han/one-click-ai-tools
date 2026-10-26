import XCTest
@testable import OctMenubarApp

final class BuildVersionMarkerTests: XCTestCase {
    // `oct menubar doctor` reads the helper's version by scanning the binary
    // for MenubarBuildVersion.versionMarker and stripping the prefix. The
    // marker therefore has to (a) carry the exact stamped version and (b)
    // stay referenced from real code — the `--version` path prints it —
    // because WMO dead-strips an unreferenced static let out of release
    // builds (official helpers up to 0.1.8-beta.1 shipped without the bytes,
    // so doctor reported them as "unknown").
    func testVersionMarkerCarriesStampedVersion() {
        XCTAssertEqual(
            MenubarBuildVersion.versionMarker,
            "oct-menubar-helper-version=\(MenubarBuildVersion.version)"
        )
    }

    func testVersionMarkerPrefixMatchesDoctorScan() {
        // Keep this in sync with menubarHelperVersionMarker in
        // cmd/menubar_helper.go; the Go probe stops at the first byte after
        // the prefix that is not printable ASCII.
        XCTAssertTrue(MenubarBuildVersion.versionMarker.hasPrefix("oct-menubar-helper-version="))
    }
}
