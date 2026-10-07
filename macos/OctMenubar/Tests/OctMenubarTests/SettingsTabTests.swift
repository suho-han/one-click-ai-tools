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
        XCTAssertEqual(
            configurationSource.components(separatedBy: "alertPercentSlider(title: \"").count - 1,
            4,
            "The main threshold plus 5h/7d/1m sub-rows must share the slider row; its 1...99 range excludes zero and 100 because the CLI payload contract rejects them."
        )
        XCTAssertTrue(
            configurationSource.contains("ThresholdSlider(value: binding, range: 1...99, color: thresholdTrackColor("),
            "Alert percent sliders must use the hand-drawn threshold slider (the macOS 26+ system slider ignores .tint) and stay within 1...99 (zero and 100 are rejected by the CLI payload contract)."
        )
        XCTAssertTrue(
            configurationSource.contains("struct ThresholdSlider"),
            "The threshold slider must draw its own track so the color coding survives control-style redesigns."
        )
        XCTAssertTrue(
            configurationSource.contains("AlertSettings.cooldownMenuChoices(current:"),
            "Cooldown must be a preset picker instead of a ±1 stepper."
        )
        XCTAssertTrue(
            configurationSource.contains("thresholdTrackColor("),
            "Threshold sliders must color their track by value."
        )
        [
            "alertEnabledBinding",
            "alertMainThresholdBinding",
            "alertCooldownMinutesBinding",
            "alertQuietChoiceBinding",
            "alertFiveHoursThresholdBinding",
            "alertSevenDaysThresholdBinding",
            "alertMonthlyThresholdBinding",
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

    func testSaveBarIsPinnedToSettingsWindowBottomOnUnsavedChanges() throws {
        let settingsSource = try sourceFile("Sources/OctMenubarApp/Views/SettingsView.swift")
        let configurationSource = try sourceFile("Sources/OctMenubarApp/Views/Settings/SettingsConfigurationTab.swift")

        // The save bar is driven by the store's unsaved-changes state and
        // lives outside the scrolling tab content, so it stays pinned to the
        // window bottom while edits are pending.
        XCTAssertTrue(settingsSource.contains("configurationStore.hasUnsavedChanges"))
        XCTAssertTrue(settingsSource.contains("unsavedChangesFooter"))
        XCTAssertTrue(settingsSource.contains("Label(\"Save changes\""))
        XCTAssertTrue(settingsSource.contains("Label(\"Revert\""))

        // The configuration tab must not embed its own save actions anymore.
        XCTAssertFalse(configurationSource.contains("saveActions"))
        XCTAssertFalse(configurationSource.contains("Save changes"))
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
