import XCTest
@testable import OctMenubarApp

final class SettingsTabTests: XCTestCase {
    func testSettingsTabsExposeConfigurationAndTools() {
        XCTAssertEqual(SettingsTab.allCases, [.configuration, .tools])
        XCTAssertEqual(SettingsTab.allCases.map(\.title), ["Configuration", "Tools"])
        XCTAssertEqual(
            SettingsTab.allCases.map(\.systemImage),
            ["slider.horizontal.3", "terminal"]
        )
    }

    func testSettingsViewDefaultsToConfiguration() throws {
        let source = try sourceFile("Sources/OctMenubarApp/Views/SettingsView.swift")

        XCTAssertTrue(source.contains("@State private var selectedTab: SettingsTab = .configuration"))
        XCTAssertFalse(source.contains("SettingsGeneralTab"))
        XCTAssertFalse(source.contains("case .general"))
    }

    func testConfigurationOwnsAppearanceAndAlertControlsWhileToolsKeepsRunAlert() throws {
        let configurationSource = try sourceFile("Sources/OctMenubarApp/Views/Settings/SettingsConfigurationTab.swift")
        let toolsSource = try sourceFile("Sources/OctMenubarApp/Views/Settings/SettingsToolsTab.swift")
        let generalTabURL = packageRoot.appendingPathComponent("Sources/OctMenubarApp/Views/Settings/SettingsGeneralTab.swift")

        XCTAssertTrue(configurationSource.contains("@AppStorage(MenubarPreferences.useProviderAccentColorsKey)"))
        XCTAssertTrue(configurationSource.contains("title: \"Appearance\""))
        XCTAssertTrue(configurationSource.contains("Toggle(\"Use provider accent colors\""))
        XCTAssertTrue(configurationSource.contains("title: \"Alerts\""))
        XCTAssertTrue(configurationSource.contains("in: 1...1_440"))
        XCTAssertEqual(
            configurationSource.components(separatedBy: "in: 1...100").count - 1,
            5,
            "Alert percent controls must exclude zero because the CLI payload contract rejects it."
        )
        [
            "alertEnabledBinding",
            "alertThresholdPercentBinding",
            "alertCriticalPercentBinding",
            "alertCooldownMinutesBinding",
            "alertQuietHoursBinding",
            "alertTimezoneBinding",
            "alertDefaultThresholdBinding",
            "alertFiveHoursThresholdBinding",
            "alertSevenDaysThresholdBinding",
        ].forEach { binding in
            XCTAssertTrue(configurationSource.contains(binding), "Missing \(binding)")
        }
        ["provider-specific", "snooze", "token"].forEach { forbiddenTerm in
            XCTAssertFalse(
                configurationSource.localizedCaseInsensitiveContains(forbiddenTerm),
                "Configuration alerts must not expose \(forbiddenTerm) controls"
            )
        }
        XCTAssertTrue(toolsSource.contains("title: \"Run alert\""))
        XCTAssertTrue(toolsSource.contains("onAction(.runAlertCheck)"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: generalTabURL.path))
    }

    func testSessionRefreshIntervalOptionsMatchSchedulerIntervals() {
        XCTAssertEqual(SessionRefreshIntervalOption.all.map(\.value), ["1h", "6h", "12h", "daily", "weekly"])
        XCTAssertTrue(SessionRefreshIntervalOption.usesHour("daily"))
        XCTAssertTrue(SessionRefreshIntervalOption.usesHour("weekly"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("12h"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("6h"))
        XCTAssertFalse(SessionRefreshIntervalOption.usesHour("1h"))
    }

    private var packageRoot: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
    }

    private func sourceFile(_ relativePath: String) throws -> String {
        try String(contentsOf: packageRoot.appendingPathComponent(relativePath))
    }
}
