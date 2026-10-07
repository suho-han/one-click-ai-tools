import Foundation
import XCTest

@testable import OctMenubarApp

final class ConfigurationScheduleTests: XCTestCase {
    private let fullSnapshotJSON = """
    {
      "config_file": "/Users/x/.oct/config.yaml",
      "menubar_title_mode": "oct",
      "menubar_refresh_interval": "1m",
      "session_refresh_enabled": true,
      "session_refresh_interval": "daily",
      "session_refresh_hour": 9,
      "agent_update_schedule": { "enabled": true, "interval": "daily", "hour": 8 },
      "agent_order": ["claude", "codex"],
      "tools": [
        { "name": "Claude Code", "binary_name": "claude", "enabled": true, "version": "2.1.3" },
        { "name": "OpenAI Codex", "binary_name": "codex", "enabled": false, "version": "" }
      ],
      "alert": {}
    }
    """

    func testSnapshotDecodesScheduleAndVersions() throws {
        let snapshot = try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(fullSnapshotJSON.utf8))

        XCTAssertEqual(snapshot.agentUpdateSchedule, AgentUpdateSchedule(enabled: true, interval: "daily", hour: 8))
        XCTAssertEqual(snapshot.tools[0].version, "2.1.3")
        // Empty version string means "probed but not installed".
        XCTAssertEqual(snapshot.tools[1].version, "")
    }

    /// Snapshots from an older oct carry neither `agent_update_schedule` nor
    /// per-tool versions; the settings must still load.
    func testSnapshotDecodesWithoutNewKeys() throws {
        let legacyJSON = """
        {
          "config_file": "/Users/x/.oct/config.yaml",
          "session_refresh_enabled": false,
          "session_refresh_interval": "daily",
          "session_refresh_hour": 9,
          "tools": [
            { "name": "Claude Code", "binary_name": "claude", "enabled": true }
          ]
        }
        """
        let snapshot = try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(legacyJSON.utf8))

        XCTAssertEqual(snapshot.agentUpdateSchedule, .off)
        XCTAssertNil(snapshot.tools[0].version, "an unprobed tool must not render a version chip")
    }

    func testDraftRoundTripsScheduleThroughPayload() throws {
        let snapshot = try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(fullSnapshotJSON.utf8))
        var draft = ConfigurationDraft(snapshot: snapshot)

        // Unchanged draft round-trips the loaded schedule.
        XCTAssertEqual(draft.updatePayload().agentUpdateSchedule, snapshot.agentUpdateSchedule)

        // Picking a new interval enables the schedule with it.
        draft.agentUpdateSchedule.setPickerValue("weekly")
        let payload = draft.updatePayload()
        XCTAssertEqual(payload.agentUpdateSchedule, AgentUpdateSchedule(enabled: true, interval: "weekly", hour: 8))
        XCTAssertTrue(payload.agentUpdateSchedule.usesHour)

        // Picking off disables and resets to the documented default hour.
        draft.agentUpdateSchedule.setPickerValue(AgentUpdateSchedule.offTag)
        XCTAssertEqual(draft.updatePayload().agentUpdateSchedule, .off)
    }

    /// The "enabled, interval unknown" state (hand-edited platform schedule)
    /// must round-trip unchanged instead of being rewritten to a guess.
    func testCustomScheduleRoundTrips() throws {
        var schedule = AgentUpdateSchedule(enabled: true, interval: "", hour: 9)

        XCTAssertEqual(schedule.pickerValue, AgentUpdateSchedule.customTag)
        XCTAssertTrue(schedule.isCustom)
        XCTAssertFalse(schedule.usesHour)

        schedule.setPickerValue(AgentUpdateSchedule.customTag)
        XCTAssertEqual(schedule, AgentUpdateSchedule(enabled: true, interval: "", hour: 9))
    }

    func testPayloadEncodingUsesSnakeCaseKeys() throws {
        let snapshot = try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(fullSnapshotJSON.utf8))
        let payload = ConfigurationDraft(snapshot: snapshot).updatePayload()

        let data = try JSONEncoder().encode(payload)
        let encoded = String(data: data, encoding: .utf8) ?? ""
        XCTAssertTrue(encoded.contains("\"agent_update_schedule\""))
        XCTAssertTrue(encoded.contains("\"enabled_tools\""))
    }
}
