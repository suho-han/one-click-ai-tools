import XCTest
@testable import OctMenubarApp

@MainActor
final class ConfigurationStoreTests: XCTestCase {
    func testHasUnsavedChangesTracksDraftAgainstSnapshot() {
        let store = ConfigurationStore(snapshot: Self.sampleSnapshot)
        XCTAssertFalse(store.hasUnsavedChanges, "A store without a draft must not report unsaved changes.")

        store.draft = ConfigurationDraft(snapshot: Self.sampleSnapshot)
        XCTAssertFalse(store.hasUnsavedChanges, "A draft matching the snapshot is not an unsaved change.")

        store.draft?.sessionRefreshEnabled.toggle()
        XCTAssertTrue(store.hasUnsavedChanges, "Any draft edit must be reported as an unsaved change.")

        store.draft = ConfigurationDraft(snapshot: Self.sampleSnapshot)
        XCTAssertFalse(store.hasUnsavedChanges, "Reverting the draft back to the snapshot clears the flag.")
    }

    func testHasUnsavedChangesDetectsEveryDraftField() {
        let store = ConfigurationStore(snapshot: Self.sampleSnapshot)
        let baseline = ConfigurationDraft(snapshot: Self.sampleSnapshot)

        var draft = baseline
        draft.menubarTitleMode = .compact
        store.draft = draft
        XCTAssertTrue(store.hasUnsavedChanges)

        draft = baseline
        draft.tools[0].enabled = false
        store.draft = draft
        XCTAssertTrue(store.hasUnsavedChanges)

        draft = baseline
        draft.alert.thresholdPercent = 70
        store.draft = draft
        XCTAssertTrue(store.hasUnsavedChanges)
    }

    private static let sampleSnapshot = ConfigurationSnapshot(
        configFile: "/tmp/oct-config.json",
        menubarTitleMode: .oct,
        sessionRefreshEnabled: false,
        sessionRefreshInterval: "daily",
        sessionRefreshHour: 9,
        tools: [ConfigTool(name: "Claude", binaryName: "claude", enabled: true)]
    )
}
