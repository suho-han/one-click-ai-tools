// Overwritten by .github/workflows/release.yml (darwin-assets) with the
// release version before `swift build -c release`; local builds report "dev".
// `oct menubar doctor` reports skew between this value and the oct binary's
// version.
enum MenubarBuildVersion {
    static let version = "dev"

    // Embedded verbatim in the binary so doctor can read the helper's
    // version by scanning the file instead of executing it: running an older
    // helper with --version would launch its status item (pre-version
    // helpers do not parse arguments) and hang until a probe timeout.
    static let versionMarker = "oct-menubar-helper-version=dev"
}
