import Foundation
import XCTest
@testable import OctMenubarApp

final class UsageSnapshotTests: XCTestCase {
    func testUsageSnapshotProjectionFromCLIResponse() throws {
        let json = #"""
        {
          "summary": {
            "total": 2,
            "ok": 1,
            "warn": 1,
            "error": 0
          },
          "results": [
            {
              "provider": "codex",
              "status": "ok",
              "used": "63.0",
              "unit": "percent",
              "buckets": {
                "5h": "63.0",
                "7d": "35.0"
              },
              "message": "Usage extracted from local Codex session logs"
            },
            {
              "provider": "opencode",
              "status": "warn",
              "used": "0",
              "unit": "percent",
              "message": "No data: No local OpenCode session logs found"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let date = Date(timeIntervalSince1970: 1_781_284_364)
        let snapshot = UsageSnapshot.from(
            response: response,
            refreshDate: date,
            refreshInterval: 60,
            timeZone: TimeZone(secondsFromGMT: 9 * 60 * 60)!
        )

        XCTAssertEqual(snapshot.statusItemTitle, "oct")
        XCTAssertEqual(snapshot.summaryLine, "2 providers · 1 ok · 1 warn · 0 error")
        XCTAssertEqual(snapshot.lastRefreshLabel, "02:12:44")
        XCTAssertEqual(snapshot.nextRefreshLabel, "02:13:44")
        XCTAssertEqual(snapshot.autoRefreshLabel, "Auto refresh: every 1m")
        XCTAssertEqual(snapshot.providers.count, 2)
        XCTAssertEqual(snapshot.providers[0], ProviderCard(name: "codex", status: .ok, metrics: [.init(label: "7d", value: "65.0%", percent: 65.0)], message: "Usage extracted from local Codex session logs"))
        XCTAssertEqual(snapshot.providers[1], ProviderCard(name: "opencode", status: .warn, metrics: [], message: "No data: No local OpenCode session logs found"))
    }

    func testUsageSnapshotSurfacesOpenCodeMonthlyBucket() throws {
        // Regression test: visibleMetrics() used to only look at "5h"/"7d",
        // so OpenCode Go's monthly quota (bucket "1m") never rendered even
        // though `oct usage --json` reports it.
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "opencode",
              "status": "ok",
              "used": "2",
              "unit": "percent",
              "buckets": {
                "5h": "2",
                "7d": "84",
                "1m": "42"
              },
              "message": "Fetched from OpenCode Go API"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)

        XCTAssertEqual(
            snapshot.providers[0].metrics,
            [.init(label: "5h", value: "98.0%", percent: 98.0), .init(label: "7d", value: "16.0%", percent: 16.0), .init(label: "1m", value: "58.0%", percent: 58.0)]
        )
    }

    func testUsageSnapshotSupportsCompactStatusItemTitle() throws {
        let json = #"""
        {
          "summary": {
            "total": 3,
            "ok": 3,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "claude-code",
              "status": "ok",
              "used": "55.0",
              "unit": "percent",
              "buckets": {
                "5h": "55.0"
              }
            },
            {
              "provider": "codex",
              "status": "ok",
              "used": "80.0",
              "unit": "percent",
              "buckets": {
                "7d": "75.0"
              }
            },
            {
              "provider": "commandcode",
              "status": "ok",
              "used": "73.3",
              "unit": "percent",
              "buckets": {
                "5h": "73.3",
                "7d": "31.3",
                "1m": "15.6"
              }
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, titleMode: .compact)

        XCTAssertEqual(snapshot.statusItemTitle, "C-45% X-25% D-27%")
        XCTAssertTrue(snapshot.statusItemAccessibilityLabel.contains("claude-code 5h 45.0%"))
        XCTAssertTrue(snapshot.statusItemAccessibilityLabel.contains("codex 7d 25.0%"))
        XCTAssertTrue(snapshot.statusItemAccessibilityLabel.contains("commandcode 5h 26.7%"))

        // The same numbers, read from the same metrics, must appear in the
        // popover card strip below the title -- this is the regression guard
        // for the bug where the compact title always showed "remaining"
        // while the card body always showed the raw "used" value beneath it.
        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "5h", value: "45.0%", percent: 45.0)])
        XCTAssertEqual(snapshot.providers[1].metrics, [.init(label: "7d", value: "25.0%", percent: 25.0)])
        XCTAssertEqual(
            snapshot.providers[2].metrics,
            [.init(label: "5h", value: "26.7%", percent: 26.7), .init(label: "7d", value: "68.7%", percent: 68.7), .init(label: "1m", value: "84.4%", percent: 84.4)]
        )
    }

    func testUsageSnapshotSummaryUsesProjectedProviderStatuses() throws {
        let json = #"""
        {
          "summary": {
            "total": 2,
            "ok": 2,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "claude-code",
              "status": "ok",
              "used": "n/a",
              "unit": "percent",
              "message": "No Claude OAuth token found"
            },
            {
              "provider": "codex",
              "status": "ok",
              "used": "14.0",
              "unit": "percent",
              "buckets": {
                "5h": "14.0",
                "7d": "13.0"
              },
              "message": "Usage extracted from local Codex session logs"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)

        XCTAssertEqual(snapshot.summaryLine, "2 providers · 1 ok · 1 warn · 0 error")
        XCTAssertEqual(snapshot.statusItemTitle, "oct")
        XCTAssertEqual(snapshot.providers.map(\.status), [.warn, .ok])
    }

    func testUsageSnapshotHidesUnknownMetricBuckets() throws {
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "codex",
              "status": "ok",
              "used": "2.0",
              "unit": "percent",
              "buckets": {
                "7d": "2.0"
              },
              "message": "Usage fetched from Codex backend API (weekly bucket)"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)

        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "7d", value: "98.0%", percent: 98.0)])
    }

    func testUsageSnapshotShowsLegacyCodexFiveHourBucketWhenWeeklyMissing() throws {
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "codex",
              "status": "ok",
              "used": "4.0",
              "unit": "percent",
              "buckets": {
                "5h": "4.0"
              },
              "message": "Usage extracted from local Codex session logs"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)

        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "5h", value: "96.0%", percent: 96.0)])
    }

    func testUsageSnapshotSurfacesModelBucketsWhenNoTimeWindowPresent() throws {
        // Regression test: Antigravity/Gemini keys its buckets "model:<name>"
        // instead of "5h"/"7d"/"1m", so visibleMetrics used to return zero
        // metrics for it even though `oct usage` (the Go table) shows them
        // via its own model-bucket fallback.
        let json = #"""
        {
          "summary": { "total": 1, "ok": 1, "warn": 0, "error": 0 },
          "results": [
            {
              "provider": "antigravity",
              "status": "ok",
              "used": "12.4",
              "unit": "percent",
              "buckets": { "model:gemini-2.5-pro": "12.4" },
              "message": "Usage fetched from Google Code Assist quota API"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)
        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "gemini-2.5-pro", value: "87.6%", percent: 87.6)])
    }

    func testUsageSnapshotSurfacesQuotaBucketWhenNoTimeWindowPresent() throws {
        // Regression test: Copilot keys its pre-computed percentage as a
        // single "quota" bucket (unit is "AIC", a count, not "percent"), so
        // visibleMetrics used to return zero metrics for it.
        let json = #"""
        {
          "summary": { "total": 1, "ok": 1, "warn": 0, "error": 0 },
          "results": [
            {
              "provider": "copilot",
              "status": "ok",
              "used": "117",
              "unit": "AIC",
              "buckets": { "quota": "58.3" },
              "message": "Usage fetched from GitHub Copilot quota API"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)
        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "quota", value: "41.7%", percent: 41.7)])
    }

    func testUsageSnapshotMapsResetCountdownsToMetrics() throws {
        // bucket_resets flows from `oct usage --json` into each metric's
        // resetsIn so the popover pill can show a clock countdown. An
        // expired reset (the "1m" bucket here) must drop the countdown
        // instead of rendering a negative duration.
        let json = #"""
        {
          "summary": { "total": 1, "ok": 1, "warn": 0, "error": 0 },
          "results": [
            {
              "provider": "claude-code",
              "status": "ok",
              "used": "55.0",
              "unit": "percent",
              "buckets": { "5h": "55.0", "7d": "35.0", "1m": "90.0" },
              "bucket_resets": {
                "5h": "2026-06-12T20:24:44Z",
                "7d": "1781889164000",
                "1m": "2026-06-12T17:00:00Z"
              },
              "message": "Usage fetched from Claude API"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        // 2026-06-12T17:12:44Z: 5h resets in 3h 12m, 7d resets in exactly 7d.
        let now = Date(timeIntervalSince1970: 1_781_284_364)
        let snapshot = UsageSnapshot.from(
            response: response,
            refreshDate: now,
            refreshInterval: 60,
            now: now
        )

        XCTAssertEqual(
            snapshot.providers[0].metrics,
            [
                .init(label: "5h", value: "45.0%", percent: 45.0, resetsIn: "3h 12m"),
                .init(label: "7d", value: "65.0%", percent: 65.0, resetsIn: "7d"),
                .init(label: "1m", value: "10.0%", percent: 10.0, resetsIn: nil),
            ]
        )
    }

    func testResetCountdownFormatsCompactDurations() {
        XCTAssertEqual(ResetCountdown.text(for: 3 * 3600 + 12 * 60), "3h 12m")
        XCTAssertEqual(ResetCountdown.text(for: 2 * 86400 + 4 * 3600), "2d 4h")
        XCTAssertEqual(ResetCountdown.text(for: 2 * 86400), "2d")
        XCTAssertEqual(ResetCountdown.text(for: 5 * 60), "5m")
        XCTAssertEqual(ResetCountdown.text(for: 30), "<1m")
        XCTAssertNil(ResetCountdown.text(for: 0))
        XCTAssertNil(ResetCountdown.text(for: -60))
    }

    func testResetCountdownParsesEpochAndRFC3339() {
        // Epoch seconds and milliseconds land on the same instant. Values
        // above 1e10 read as milliseconds, mirroring Go's threshold.
        XCTAssertEqual(ResetCountdown.parseResetTime("1781284364"), Date(timeIntervalSince1970: 1_781_284_364))
        XCTAssertEqual(ResetCountdown.parseResetTime("1781284364000"), Date(timeIntervalSince1970: 1_781_284_364))
        XCTAssertEqual(ResetCountdown.parseResetTime("1970-01-01T00:16:40Z"), Date(timeIntervalSince1970: 1000))
        XCTAssertNil(ResetCountdown.parseResetTime("not-a-time"))
        XCTAssertNil(ResetCountdown.parseResetTime(nil))
        XCTAssertNil(ResetCountdown.parseResetTime(""))
        // 900s of remaining time formats as "15m" through the full pipeline.
        XCTAssertEqual(ResetCountdown.label(until: "1781284364", now: Date(timeIntervalSince1970: 1_781_283_464)), "15m")
    }

    func testCompactMetricValuePrefersResolvedPercent() {
        let card = ProviderCard(
            name: "claude-code",
            status: .ok,
            metrics: [.init(label: "5h", value: "45.0%", percent: 45.4)],
            message: nil
        )
        XCTAssertEqual(card.compactMetricValue, "45%")
    }

    func testUsageRowMetricsWidthsFitLongestContent() {
        let providers = [
            ProviderCard(name: "antigravity", status: .ok, metrics: [
                .init(label: "Claude/GPT", value: "100.0%", percent: 100.0, resetsIn: "6d 23h"),
                .init(label: "Gemini", value: "73.0%", percent: 73.0, resetsIn: "6d 1h"),
            ], message: nil),
            ProviderCard(name: "codex", status: .ok, metrics: [
                .init(label: "7d", value: "31.0%", percent: 31.0, resetsIn: "3d 4h"),
            ], message: nil),
        ]

        let widths = UsageRowMetrics.columnWidths(for: providers)
        // Every column is at least as wide as its longest string across all
        // providers, so no text can truncate; the countdown column accounts
        // for the "/ " prefix.
        XCTAssertGreaterThanOrEqual(widths.label, UsageRowMetrics.width(of: ["Claude/GPT"]))
        XCTAssertGreaterThanOrEqual(widths.value, UsageRowMetrics.width(of: ["100.0%"]))
        XCTAssertGreaterThanOrEqual(widths.countdown, UsageRowMetrics.width(of: ["/ 6d 23h"]))
        // Widths track content: a longer label measures wider than a short one.
        XCTAssertGreaterThan(UsageRowMetrics.width(of: ["Claude/GPT"]), UsageRowMetrics.width(of: ["7d"]))
        // Empty input collapses to zero so absent columns don't eat bar width.
        XCTAssertEqual(UsageRowMetrics.width(of: []), 0)
    }

    func testUsageSnapshotKeepsPlanOutOfProviderMessage() throws {
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "codex",
              "plan": "plus",
              "plan_source": "local logs",
              "status": "ok",
              "used": "15.0",
              "unit": "percent",
              "buckets": {
                "5h": "15.0",
                "7d": "10.0"
              },
              "message": "Usage extracted from local Codex session logs"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)

        XCTAssertEqual(snapshot.providers[0].plan, "plus")
        XCTAssertEqual(snapshot.providers[0].message, "Usage extracted from local Codex session logs")
        XCTAssertFalse(snapshot.providers[0].message?.contains("Plan:") ?? true)
    }

    func testConfigurationSnapshotDecodesConfigListJSON() throws {
        let json = #"""
        {
          "config_file": "/Users/me/.oct/config.yaml",
          "menubar_title_mode": "compact",
          "session_refresh_enabled": true,
          "session_refresh_interval": "weekly",
          "session_refresh_hour": 9,
          "alert": {
            "enabled": true,
            "threshold_percent": 82.5,
            "critical_percent": 97.5,
            "cooldown_minutes": 120,
            "quiet_hours": "00:00-08:00",
            "timezone": "Asia/Seoul",
            "thresholds": {
              "default": 81,
              "5h": 82.5,
              "7d": 90
            }
          },
          "tools": [
            {
              "name": "OpenAI Codex",
              "binary_name": "codex",
              "enabled": true
            },
            {
              "name": "Command Code",
              "binary_name": "commandcode",
              "enabled": true
            },
            {
              "name": "Claude Code",
              "binary_name": "claude",
              "enabled": false
            }
          ]
        }
        """#

        let snapshot = try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(json.utf8))

        XCTAssertEqual(snapshot.configFile, "/Users/me/.oct/config.yaml")
        XCTAssertEqual(snapshot.menubarTitleMode, .compact)
        XCTAssertTrue(snapshot.sessionRefreshEnabled)
        XCTAssertEqual(snapshot.sessionRefreshInterval, "weekly")
        XCTAssertEqual(snapshot.sessionRefreshHour, 9)
        XCTAssertEqual(snapshot.alert, AlertSettings(
            enabled: true,
            thresholdPercent: 82.5,
            criticalPercent: 97.5,
            cooldownMinutes: 120,
            quietHours: "00:00-08:00",
            timezone: "Asia/Seoul",
            thresholds: AlertThresholds(defaultThreshold: 81, fiveHours: 82.5, sevenDays: 90)
        ))
        XCTAssertEqual(snapshot.tools.map(\.binaryName), ["codex", "commandcode", "claude"])
        XCTAssertEqual(snapshot.tools.map(\.enabled), [true, true, false])
    }

    func testConfigurationSnapshotDecodesLegacyConfigListJSON() throws {
        let json = #"""
        {
          "config_file": "/Users/me/.oct/config.yaml",
          "session_refresh_enabled": true,
          "session_refresh_interval": "weekly",
          "session_refresh_hour": 9,
          "tools": [
            {
              "name": "OpenAI Codex",
              "binary_name": "codex",
              "enabled": true
            }
          ]
        }
        """#

        let snapshot = try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(json.utf8))

        XCTAssertEqual(snapshot.configFile, "/Users/me/.oct/config.yaml")
        XCTAssertEqual(snapshot.menubarTitleMode, .oct)
        XCTAssertEqual(snapshot.alert, .goDefaults)
        XCTAssertEqual(snapshot.tools.map(\.binaryName), ["codex"])
    }

    func testConfigurationDraftBuildsUpdatePayloadAfterEdits() throws {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/config.yaml",
            menubarTitleMode: .oct,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true),
                ConfigTool(name: "Command Code", binaryName: "commandcode", enabled: false),
                ConfigTool(name: "Claude Code", binaryName: "claude", enabled: false),
            ]
        )
        var draft = ConfigurationDraft(snapshot: snapshot)

        draft.setTool("claude", enabled: true)
        draft.setTool("commandcode", enabled: true)
        draft.sessionRefreshEnabled = true
        draft.sessionRefreshInterval = "weekly"
        draft.sessionRefreshHour = 22
        draft.moveTool("claude", by: -1)
        draft.alert.enabled = true
        draft.alert.thresholdPercent = 82.5
        draft.alert.criticalPercent = 97.5
        draft.alert.cooldownMinutes = 120
        draft.alert.quietHours = "00:00-08:00"
        draft.alert.timezone = "Asia/Seoul"
        draft.alert.thresholds.defaultThreshold = 81
        draft.alert.thresholds.fiveHours = 82.5
        draft.alert.thresholds.sevenDays = 90

        let payload = draft.updatePayload()

        XCTAssertEqual(payload.enabledTools, ["codex", "claude", "commandcode"])
        XCTAssertEqual(payload.menubarTitleMode, .oct)
        XCTAssertTrue(payload.sessionRefreshEnabled)
        XCTAssertEqual(payload.sessionRefreshInterval, "weekly")
        XCTAssertEqual(payload.sessionRefreshHour, 22)
        XCTAssertEqual(payload.agentOrder, ["codex", "claude", "commandcode"])
        XCTAssertTrue(payload.alert.enabled)
        XCTAssertEqual(payload.alert.thresholdPercent, 82.5)
        XCTAssertEqual(payload.alert.criticalPercent, 97.5)
        XCTAssertEqual(payload.alert.cooldownMinutes, 120)
        XCTAssertEqual(payload.alert.quietHours, "00:00-08:00")
        XCTAssertEqual(payload.alert.timezone, "Asia/Seoul")
        XCTAssertEqual(payload.alert.thresholds.defaultThreshold, 81)
        XCTAssertEqual(payload.alert.thresholds.fiveHours, 82.5)
        XCTAssertEqual(payload.alert.thresholds.sevenDays, 90)

        let encoded = try JSONEncoder().encode(payload)
        let json = try JSONSerialization.jsonObject(with: encoded) as! [String: Any]
        let alert = try XCTUnwrap(json["alert"] as? [String: Any])
        let thresholds = try XCTUnwrap(alert["thresholds"] as? [String: Any])
        XCTAssertEqual(Set(json.keys), [
            "enabled_tools",
            "menubar_title_mode",
            "session_refresh_enabled",
            "session_refresh_interval",
            "session_refresh_hour",
            "agent_order",
            "alert",
        ])
        XCTAssertEqual(Set(alert.keys), [
            "enabled",
            "threshold_percent",
            "critical_percent",
            "cooldown_minutes",
            "quiet_hours",
            "timezone",
            "thresholds",
        ])
        XCTAssertEqual(Set(thresholds.keys), ["default", "5h", "7d"])
        XCTAssertEqual(json["enabled_tools"] as? [String], ["codex", "claude", "commandcode"])
        XCTAssertEqual(json["menubar_title_mode"] as? String, "oct")
        XCTAssertEqual(json["session_refresh_enabled"] as? Bool, true)
        XCTAssertEqual(json["session_refresh_interval"] as? String, "weekly")
        XCTAssertEqual(json["session_refresh_hour"] as? Int, 22)
        XCTAssertEqual(json["agent_order"] as? [String], ["codex", "claude", "commandcode"])
        XCTAssertEqual(alert["enabled"] as? Bool, true)
        XCTAssertEqual(alert["threshold_percent"] as? Double, 82.5)
        XCTAssertEqual(alert["critical_percent"] as? Double, 97.5)
        XCTAssertEqual(alert["cooldown_minutes"] as? Int, 120)
        XCTAssertEqual(alert["quiet_hours"] as? String, "00:00-08:00")
        XCTAssertEqual(alert["timezone"] as? String, "Asia/Seoul")
        XCTAssertEqual(thresholds["default"] as? Double, 81)
        XCTAssertEqual(thresholds["5h"] as? Double, 82.5)
        XCTAssertEqual(thresholds["7d"] as? Double, 90)
    }

    func testConfigurationSnapshotRejectsMalformedAlertJSON() {
        let json = #"""
        {
          "config_file": "/tmp/config.yaml",
          "session_refresh_enabled": false,
          "session_refresh_interval": "daily",
          "session_refresh_hour": 9,
          "tools": [],
          "alert": {
            "enabled": "true",
            "threshold_percent": 80,
            "critical_percent": 98,
            "cooldown_minutes": 360,
            "quiet_hours": "",
            "timezone": "",
            "thresholds": { "default": 80, "5h": 80, "7d": 80 }
          }
        }
        """#

        XCTAssertThrowsError(try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(json.utf8))) { error in
            guard case DecodingError.typeMismatch = error else {
                return XCTFail("Expected a type mismatch, got \(error)")
            }
        }
    }

    func testConfigurationDraftRevertsToLoadedSnapshot() {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/config.yaml",
            menubarTitleMode: .compact,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true),
            ]
        )
        var draft = ConfigurationDraft(snapshot: snapshot)
        draft.menubarTitleMode = .oct
        draft.setTool("codex", enabled: false)

        draft.revert(to: snapshot)

        XCTAssertEqual(draft.menubarTitleMode, .compact)
        XCTAssertEqual(draft.tools.map(\.enabled), [true])
    }

    func testConfigurationDraftPersistsMenubarTitleModeSelection() throws {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/config.yaml",
            menubarTitleMode: .oct,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true),
            ]
        )
        var draft = ConfigurationDraft(snapshot: snapshot)

        draft.setMenubarTitleMode(.compact)
        let payload = draft.updatePayload()
        let encoded = try JSONEncoder().encode(payload)
        let decoded = try JSONDecoder().decode(ConfigurationUpdatePayload.self, from: encoded)

        XCTAssertEqual(draft.menubarTitleMode, .compact)
        XCTAssertEqual(decoded.menubarTitleMode, .compact)
        XCTAssertEqual(MenubarTitleMode.compact.label, "Compact %")
    }

    func testResolveExecutablePrefersExplicitOverride() throws {
        let temp = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent(UUID().uuidString)
        let override = temp.appendingPathComponent("custom-oct")
        try FileManager.default.createDirectory(at: temp, withIntermediateDirectories: true)
        FileManager.default.createFile(atPath: override.path, contents: Data())
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: override.path)

        let resolution = OctCLIService.resolveExecutable(
            environment: ["OCT_MENUBAR_OCT_PATH": override.path],
            currentDirectoryURL: temp,
            processExecutableURL: temp.appendingPathComponent("OctMenubarApp")
        )

        XCTAssertEqual(resolution.url.standardizedFileURL.path, override.standardizedFileURL.path)
    }

    func testResolveExecutableWalksAncestorDirectories() throws {
        let temp = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent(UUID().uuidString)
        let repoRoot = temp.appendingPathComponent("repo")
        let workingDir = repoRoot.appendingPathComponent("macos/OctMenubar")
        let oct = repoRoot.appendingPathComponent("oct")
        try FileManager.default.createDirectory(at: workingDir, withIntermediateDirectories: true)
        FileManager.default.createFile(atPath: oct.path, contents: Data())
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: oct.path)

        let resolution = OctCLIService.resolveExecutable(
            environment: [:],
            currentDirectoryURL: workingDir,
            processExecutableURL: workingDir.appendingPathComponent(".build/debug/OctMenubarApp")
        )

        XCTAssertEqual(resolution.url.standardizedFileURL.path, oct.standardizedFileURL.path)
        XCTAssertTrue(resolution.searchedPaths.contains(oct.standardizedFileURL.path))
    }

    @MainActor
    func testPopoverPreferredSizeCapsHeightForScrollableContent() {
        let compact = PopoverView.preferredSize(for: 2)
        XCTAssertEqual(compact.width, 640)
        XCTAssertLessThan(compact.height, 620)

        let crowded = PopoverView.preferredSize(for: 12)
        XCTAssertEqual(crowded.width, 640)
        XCTAssertEqual(crowded.height, 620)
    }

    func testSettingsActionUsesSwiftUISettingsLink() throws {
        let testFile = URL(fileURLWithPath: #filePath)
        let packageRoot = testFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let footerPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/Views/FooterActionsView.swift")
        let popoverPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/PopoverView.swift")
        let footerSource = try String(contentsOf: footerPath, encoding: .utf8)
        let popoverSource = try String(contentsOf: popoverPath, encoding: .utf8)

        XCTAssertTrue(footerSource.contains("SettingsLink"), "Settings action should use SwiftUI SettingsLink")
        XCTAssertFalse(popoverSource.contains("showSettingsWindow"), "Settings action should not rely on AppKit showSettingsWindow selector")
    }

    func testProviderCardShowsPlanInHeaderWithoutPlanStrip() throws {
        let testFile = URL(fileURLWithPath: #filePath)
        let packageRoot = testFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let providerPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/Views/ProviderCardView.swift")
        let source = try String(contentsOf: providerPath, encoding: .utf8)

        XCTAssertTrue(source.contains("provider.plan"), "Provider card should still render the plan value")
        XCTAssertFalse(source.contains("Text(\"PLAN\")"), "Provider card should not render the old full-width PLAN label")
        XCTAssertFalse(source.contains("private var planStrip"), "Provider card should remove the old full-width plan strip")
    }

    func testRefreshMetadataRendersBelowProviderSection() throws {
        let testFile = URL(fileURLWithPath: #filePath)
        let packageRoot = testFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let headerPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/Views/HeaderView.swift")
        let popoverPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/PopoverView.swift")
        let headerSource = try String(contentsOf: headerPath, encoding: .utf8)
        let popoverSource = try String(contentsOf: popoverPath, encoding: .utf8)

        XCTAssertFalse(headerSource.contains("Last refresh"), "Header should not own refresh metadata")
        XCTAssertTrue(popoverSource.contains("refreshMetadataSection"), "Popover should render refresh metadata below provider usage")
        XCTAssertLessThan(
            popoverSource.range(of: "providerSection")!.lowerBound,
            popoverSource.range(of: "refreshMetadataSection")!.lowerBound
        )
        XCTAssertLessThan(
            popoverSource.range(of: "refreshMetadataSection")!.lowerBound,
            popoverSource.range(of: "FooterActionsView")!.lowerBound
        )
    }

    func testPlaceholderSnapshotFlagsInitialLoadingState() {
        XCTAssertTrue(UsageSnapshot.placeholder.isPlaceholder, "launch placeholder should be flagged as the initial loading state")
        XCTAssertFalse(UsageSnapshot.error(message: "boom").isPlaceholder, "error snapshot must not be treated as the launch placeholder")

        let loaded = UsageSnapshot(
            statusItemTitle: "oct",
            statusItemAccessibilityLabel: "oct usage",
            title: "Usage Overview",
            summaryLine: "ok",
            lastRefreshLabel: "just now",
            nextRefreshLabel: "in 1m",
            autoRefreshLabel: "Auto refresh: every 1m",
            providers: [
                ProviderCard(name: "codex", status: .ok, metrics: [], message: nil),
            ],
            note: nil
        )
        XCTAssertFalse(loaded.isPlaceholder, "a refreshed snapshot must not be flagged as placeholder")
    }

    func testProvidersHeaderShowsSpinnerWhilePlaceholderLoads() throws {
        let testFile = URL(fileURLWithPath: #filePath)
        let packageRoot = testFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let popoverPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/PopoverView.swift")
        let popoverSource = try String(contentsOf: popoverPath, encoding: .utf8)

        XCTAssertTrue(popoverSource.contains("snapshot.isPlaceholder"), "provider header should branch on the initial loading state")
        XCTAssertTrue(popoverSource.contains("ProgressView()"), "provider header should render a loading spinner while placeholder")
    }
}
