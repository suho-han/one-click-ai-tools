import XCTest

@testable import OctMenubarApp

final class ConfigurationDurationTests: XCTestCase {
    func testParseGoDurationTable() {
        let table: [(raw: String, expected: TimeInterval?)] = [
            ("90s", 90),
            ("1m", 60),
            ("1m30s", 90),
            ("2h", 7200),
            ("500ms", 0.5),
            ("1.5h", 5400),
            (" 1m ", 60),
            // Invalid or non-positive input falls back (nil → 60s default).
            ("", nil),
            ("0s", nil),
            ("-30s", nil),
            ("bogus", nil),
            ("1x", nil),
            ("m", nil),
        ]
        for entry in table {
            XCTAssertEqual(
                ConfigurationSnapshot.parseGoDuration(entry.raw),
                entry.expected,
                "parseGoDuration(\(entry.raw))"
            )
        }
    }

    func testSnapshotRefreshIntervalDefaultsTo60Seconds() {
        // Legacy payload without menubar_refresh_interval decodes with the
        // default and maps to 60s.
        let json = """
        {
          "config_file": "/tmp/config.yaml",
          "session_refresh_enabled": false,
          "session_refresh_interval": "daily",
          "session_refresh_hour": 9,
          "tools": []
        }
        """
        let snapshot = try! JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(json.utf8))
        XCTAssertEqual(snapshot.menubarRefreshInterval, "1m")
        XCTAssertEqual(snapshot.refreshInterval, 60)
    }

    func testSnapshotRefreshIntervalHonorsConfiguredValue() {
        let json = """
        {
          "config_file": "/tmp/config.yaml",
          "menubar_refresh_interval": "90s",
          "session_refresh_enabled": false,
          "session_refresh_interval": "daily",
          "session_refresh_hour": 9,
          "tools": []
        }
        """
        let snapshot = try! JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(json.utf8))
        XCTAssertEqual(snapshot.refreshInterval, 90)

        // Invalid value falls back to 60s.
        let badJSON = json.replacingOccurrences(of: "90s", with: "bogus")
        let bad = try! JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(badJSON.utf8))
        XCTAssertEqual(bad.refreshInterval, 60)
    }
}
