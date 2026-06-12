import Foundation
import Testing
@testable import OctMenubarApp

struct UsageSnapshotTests {
    @Test
    func usageSnapshotProjectionFromCLIResponse() throws {
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
        let snapshot = UsageSnapshot.from(response: response, refreshDate: date, refreshInterval: 60)

        #expect(snapshot.summaryLine == "2 providers · 1 ok · 1 warn · 0 error")
        #expect(snapshot.lastRefreshLabel == "15:26:04")
        #expect(snapshot.nextRefreshLabel == "15:27:04")
        #expect(snapshot.providers.count == 2)
        #expect(snapshot.providers[0] == ProviderCard(name: "codex", status: "ok", metricsLine: "5h 63.0 · 7d 35.0", message: "Usage extracted from local Codex session logs"))
        #expect(snapshot.providers[1] == ProviderCard(name: "opencode", status: "warn", metricsLine: "5h - · 7d -", message: "No data: No local OpenCode session logs found"))
    }
}
