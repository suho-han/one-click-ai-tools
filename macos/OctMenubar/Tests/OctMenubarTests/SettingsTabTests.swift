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
}
