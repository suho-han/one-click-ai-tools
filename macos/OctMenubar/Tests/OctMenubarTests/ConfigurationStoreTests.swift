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

        store.draft?.telemetryEnabled = false
        XCTAssertTrue(store.hasUnsavedChanges, "Telemetry changes must activate Save/Revert.")
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

    func testTelemetryOptOutSurvivesSaveAndReload() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("oct-telemetry-config-\(UUID())")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let cli = directory.appendingPathComponent("oct")
        let payloadFile = directory.appendingPathComponent("payload.json")
        let initial = #"{"config_file":"/tmp/oct-config.json","session_refresh_enabled":false,"session_refresh_interval":"daily","session_refresh_hour":9,"tools":[{"name":"Codex","binary_name":"codex","enabled":true}]}"#
        let saved = #"{"config_file":"/tmp/oct-config.json","telemetry_enabled":false,"session_refresh_enabled":false,"session_refresh_interval":"daily","session_refresh_hour":9,"tools":[{"name":"Codex","binary_name":"codex","enabled":true}]}"#
        let script = """
        #!/bin/sh
        if [ "$1" = "config" ] && [ "$2" = "update" ]; then
            printf '%s' "$4" > '\(payloadFile.path)'
            exit 0
        fi
        if [ -f '\(payloadFile.path)' ]; then
            printf '%s\\n' '\(saved)'
        else
            printf '%s\\n' '\(initial)'
        fi
        """
        try script.write(to: cli, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cli.path)
        let store = ConfigurationStore(service: OctCLIService(executableURL: cli, processTimeout: 5))

        await store.loadDraft()
        XCTAssertTrue(store.draft?.telemetryEnabled == true)
        store.draft?.telemetryEnabled = false
        XCTAssertTrue(store.hasUnsavedChanges)
        store.revertDraft()
        XCTAssertTrue(store.draft?.telemetryEnabled == true)
        store.draft?.telemetryEnabled = false
        await store.saveDraft()

        XCTAssertEqual(store.feedback?.message, "Saved.")
        XCTAssertFalse(try XCTUnwrap(store.snapshot).telemetryEnabled)
        XCTAssertFalse(try XCTUnwrap(store.draft).telemetryEnabled)
        XCTAssertFalse(store.hasUnsavedChanges)
        let payloadData = try Data(contentsOf: payloadFile)
        let payload = try JSONSerialization.jsonObject(with: payloadData) as! [String: Any]
        XCTAssertEqual(payload["telemetry_enabled"] as? Bool, false)

        await store.loadDraft()
        XCTAssertFalse(try XCTUnwrap(store.draft).telemetryEnabled)
    }

    func testConfigurationProcessPreservesInheritedEnvironmentAndMarksMenubarSource() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("oct-telemetry-env-\(UUID())")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let cli = directory.appendingPathComponent("oct")
        let environmentFile = directory.appendingPathComponent("environment.txt")
        let script = """
        #!/bin/sh
        printf '%s\\n' "$OCT_TELEMETRY_SOURCE" "$HOME" "$PATH" "$DO_NOT_TRACK" "$OCT_TELEMETRY_ENABLED" > '\(environmentFile.path)'
        printf '%s\\n' '{"config_file":"/tmp/oct-config.json","session_refresh_enabled":false,"session_refresh_interval":"daily","session_refresh_hour":9,"tools":[]}'
        """
        try script.write(to: cli, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cli.path)

        _ = try await OctCLIService(executableURL: cli).fetchConfigurationSnapshot()
        let captured = try String(contentsOf: environmentFile, encoding: .utf8)
            .split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        let parent = ProcessInfo.processInfo.environment
        XCTAssertEqual(Array(captured.prefix(5)), [
            "menubar", parent["HOME"] ?? "", parent["PATH"] ?? "",
            parent["DO_NOT_TRACK"] ?? "", parent["OCT_TELEMETRY_ENABLED"] ?? "",
        ])
    }

    func testLoadedFeedbackDisappearsAfterFiveSeconds() async throws {
        let cli = try makeConfigurationCLI()
        defer { try? FileManager.default.removeItem(at: cli.deletingLastPathComponent()) }
        let store = ConfigurationStore(service: OctCLIService(executableURL: cli, processTimeout: 5))

        await store.loadDraft()
        XCTAssertEqual(store.feedback?.message, "Loaded configuration.")
        try await Task.sleep(for: .seconds(5.3))
        XCTAssertNil(store.feedback)
        XCTAssertNotNil(store.draft)
    }

    func testLoadedFeedbackTimeoutDoesNotDismissNewerFeedback() async throws {
        let cli = try makeConfigurationCLI()
        defer { try? FileManager.default.removeItem(at: cli.deletingLastPathComponent()) }
        let store = ConfigurationStore(service: OctCLIService(executableURL: cli, processTimeout: 5))

        await store.loadDraft()
        try await Task.sleep(for: .seconds(2))
        store.revertDraft()
        try await Task.sleep(for: .seconds(3.3))
        XCTAssertEqual(store.feedback?.message, "Reverted to the last loaded configuration.")
    }

    private func makeConfigurationCLI() throws -> URL {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("oct-feedback-\(UUID())")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        let cli = directory.appendingPathComponent("oct")
        let script = #"""
        #!/bin/sh
        printf '%s\n' '{"config_file":"/tmp/oct-config.json","session_refresh_enabled":false,"session_refresh_interval":"daily","session_refresh_hour":9,"tools":[{"name":"Codex","binary_name":"codex","enabled":true}]}'
        """#
        try script.write(to: cli, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cli.path)
        return cli
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
