import XCTest
@testable import OctMenubarApp

final class SettingsTabTests: XCTestCase {
    func testSessionRefreshIntervalOptionsMatchSchedulerIntervals() {
        XCTAssertEqual(SessionRefreshIntervalOption.all.map(\.value), ["1h", "6h", "12h", "daily", "weekly"])
        XCTAssertTrue(SessionRefreshIntervalOption.usesHour("daily"))
        XCTAssertTrue(SessionRefreshIntervalOption.usesHour("weekly"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("12h"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("6h"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("1h"))
    }

    func testCooldownPickerChoicesStayWithinCLIPayloadContract() {
        // The CLI rejects cooldown_minutes <= 0; the picker presets stay in
        // the 1...1_440 range the old stepper enforced.
        XCTAssertFalse(AlertSettings.cooldownChoices.isEmpty)
        XCTAssertTrue(AlertSettings.cooldownChoices.allSatisfy { (1...1_440).contains($0) })
        XCTAssertTrue(AlertSettings.cooldownChoices.contains(360), "The Go default cooldown must be a preset.")

        // A CLI-written custom duration joins the menu so the selection keeps
        // a matching tag; presets stay sorted around it.
        XCTAssertEqual(
            AlertSettings.cooldownMenuChoices(current: 90),
            [15, 30, 60, 90, 120, 180, 360, 720, 1440]
        )
        XCTAssertEqual(AlertSettings.cooldownMenuChoices(current: 360), AlertSettings.cooldownChoices)
        XCTAssertEqual(AlertSettings.cooldownMenuChoices(current: 0), AlertSettings.cooldownChoices)

        XCTAssertEqual(AlertSettings.cooldownLabel(for: 15), "15 min")
        XCTAssertEqual(AlertSettings.cooldownLabel(for: 60), "1 hr")
        XCTAssertEqual(AlertSettings.cooldownLabel(for: 90), "1.5 hr")
        XCTAssertEqual(AlertSettings.cooldownLabel(for: 1440), "24 hr")
    }
}
