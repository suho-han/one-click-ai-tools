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

        XCTAssertEqual(snapshot.statusItemTitle, "oct !")
        XCTAssertEqual(snapshot.summaryLine, "2 providers · 1 ok · 1 warn · 0 error")
        XCTAssertEqual(snapshot.lastRefreshLabel, "02:12:44")
        XCTAssertEqual(snapshot.nextRefreshLabel, "02:13:44")
        XCTAssertEqual(snapshot.autoRefreshLabel, "Auto refresh: every 1m")
        XCTAssertEqual(snapshot.providers.count, 2)
        XCTAssertEqual(snapshot.providers[0], ProviderCard(name: "codex", status: .ok, metrics: [.init(label: "5h", value: "63.0"), .init(label: "7d", value: "35.0")], message: "Usage extracted from local Codex session logs"))
        XCTAssertEqual(snapshot.providers[1], ProviderCard(name: "opencode", status: .warn, metrics: [.init(label: "5h", value: "-"), .init(label: "7d", value: "-")], message: "No data: No local OpenCode session logs found"))
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
        XCTAssertEqual(snapshot.statusItemTitle, "oct !")
        XCTAssertEqual(snapshot.providers.map(\.status), [.warn, .ok])
    }

    func testConfigurationSnapshotDecodesConfigListJSON() throws {
        let json = #"""
        {
          "config_file": "/Users/me/.oct/config.yaml",
          "usage_display_mode": "remaining",
          "session_refresh_enabled": true,
          "session_refresh_interval": "weekly",
          "session_refresh_hour": 9,
          "tools": [
            {
              "name": "OpenAI Codex",
              "binary_name": "codex",
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
        XCTAssertEqual(snapshot.usageDisplayMode, .remaining)
        XCTAssertTrue(snapshot.sessionRefreshEnabled)
        XCTAssertEqual(snapshot.sessionRefreshInterval, "weekly")
        XCTAssertEqual(snapshot.sessionRefreshHour, 9)
        XCTAssertEqual(snapshot.tools.map(\.binaryName), ["codex", "claude"])
        XCTAssertEqual(snapshot.tools.map(\.enabled), [true, false])
    }

    func testConfigurationDraftBuildsUpdatePayloadAfterEdits() throws {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/config.yaml",
            usageDisplayMode: .remaining,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true),
                ConfigTool(name: "Claude Code", binaryName: "claude", enabled: false),
            ]
        )
        var draft = ConfigurationDraft(snapshot: snapshot)

        draft.setTool("claude", enabled: true)
        draft.usageDisplayMode = .used
        draft.sessionRefreshEnabled = true
        draft.sessionRefreshInterval = "weekly"
        draft.sessionRefreshHour = 22

        let payload = draft.updatePayload()

        XCTAssertEqual(payload.enabledTools, ["codex", "claude"])
        XCTAssertEqual(payload.usageDisplayMode, .used)
        XCTAssertTrue(payload.sessionRefreshEnabled)
        XCTAssertEqual(payload.sessionRefreshInterval, "weekly")
        XCTAssertEqual(payload.sessionRefreshHour, 22)
    }

    func testConfigurationDraftRevertsToLoadedSnapshot() {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/config.yaml",
            usageDisplayMode: .remaining,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true),
            ]
        )
        var draft = ConfigurationDraft(snapshot: snapshot)
        draft.usageDisplayMode = .used
        draft.setTool("codex", enabled: false)

        draft.revert(to: snapshot)

        XCTAssertEqual(draft.usageDisplayMode, .remaining)
        XCTAssertEqual(draft.tools.map(\.enabled), [true])
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
}
