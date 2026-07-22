import XCTest
@testable import OctMenubarApp

final class SettingsTabTests: XCTestCase {
    func testSettingsTabsExposeTheThreeUserTasks() {
        XCTAssertEqual(SettingsTab.allCases, [.general, .configuration, .tools])
        XCTAssertEqual(SettingsTab.allCases.map(\.title), ["General", "Configuration", "Tools"])
        XCTAssertEqual(
            SettingsTab.allCases.map(\.systemImage),
            ["paintbrush", "slider.horizontal.3", "terminal"]
        )
    }

    func testSessionRefreshIntervalOptionsMatchSchedulerIntervals() {
        XCTAssertEqual(SessionRefreshIntervalOption.all.map(\.value), ["1h", "6h", "12h", "daily", "weekly"])
        XCTAssertTrue(SessionRefreshIntervalOption.usesHour("daily"))
        XCTAssertTrue(SessionRefreshIntervalOption.usesHour("weekly"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("12h"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("6h"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("1h"))
    }
}
