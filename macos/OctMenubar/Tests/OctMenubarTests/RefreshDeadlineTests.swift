import XCTest

@testable import OctMenubarApp

/// Regression tests for the stuck-refresh incident: a single hung
/// `oct usage --json` left the view model's isRefreshing true forever, so
/// the menubar never refreshed again, and every wait inside runProcess could
/// stall indefinitely (SIGTERM trapped by the child, a grandchild holding the
/// pipes, or Foundation losing the termination notification).
@MainActor
final class RefreshDeadlineTests: XCTestCase {
    // MARK: - Stub helpers

    private func writeExecutableStub(_ body: String) throws -> URL {
        let url = FileManager.default.temporaryDirectory
            .appendingPathComponent("oct-menubar-test-\(UUID().uuidString)")
            .appendingPathExtension("sh")
        try "#!/bin/sh\n\(body)\n".write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        addTeardownBlock { try? FileManager.default.removeItem(at: url) }
        return url
    }

    private static let validUsageJSON =
        #"echo '{"summary":{"total":1,"ok":1,"warn":0,"error":0},"results":[{"provider":"codex","status":"ok","used":"12.5","unit":"percent"}]}'"#

    private func waitFor(_ seconds: TimeInterval, _ condition: () -> Bool) async -> Bool {
        let deadline = Date().addingTimeInterval(seconds)
        while Date() < deadline {
            if condition() { return true }
            try? await Task.sleep(nanoseconds: 50_000_000)
        }
        return condition()
    }

    // MARK: - OctCLIService deadlines

    func testRefreshDeadlineDefaultsAboveProcessBudget() {
        XCTAssertEqual(OctCLIService().refreshDeadline, 30, "20s process timeout + 2x2s grace + 6s headroom")
        XCTAssertEqual(OctCLIService(processTimeout: 5).refreshDeadline, 15)
        XCTAssertEqual(
            OctCLIService(processTimeout: 5, refreshDeadline: 3).refreshDeadline, 3,
            "an explicit deadline must win over the derived default")
    }

    /// The child traps SIGTERM and its `sleep` grandchild keeps the pipes
    /// open: EOF never arrives, SIGTERM is ignored. runProcess must still
    /// fail bounded (EOF-wait deadline, SIGKILL escalation) instead of
    /// blocking forever in waitUntilExit.
    func testTermImmuneChildWithHeldPipesFailsBounded() async throws {
        let stub = try writeExecutableStub("trap '' TERM\nsleep 30")
        let service = OctCLIService(
            executableURL: stub,
            processTimeout: 0.3,
            refreshDeadline: 1.0
        )

        let start = Date()
        do {
            _ = try await service.fetchUsageSnapshot(configuration: nil)
            XCTFail("fetchUsageSnapshot must fail for a child that never terminates cleanly")
        } catch {
            // Expected: SIGKILL escalation or the overall deadline.
        }
        XCTAssertLessThan(Date().timeIntervalSince(start), 10, "the failure must be deadline-bounded, not a hang")
    }

    func testWellBehavedChildOutputStillDecodes() async throws {
        let stub = try writeExecutableStub(Self.validUsageJSON)
        let service = OctCLIService(executableURL: stub)

        let snapshot = try await service.fetchUsageSnapshot(configuration: nil)

        XCTAssertEqual(snapshot.providers.first?.name, "codex")
        XCTAssertEqual(snapshot.providers.first?.status, .ok)
    }

    // MARK: - UsageViewModel completion deadline

    /// The service-level watchdog is disabled (huge processTimeout) and the
    /// child sleeps forever holding the pipes, so only the view model's own
    /// completion deadline can recover. A late-landing service result (the
    /// settle path eventually SIGKILLs the child) must be discarded by the
    /// generation guard instead of re-publishing.
    func testHungRefreshIsRecoveredByViewModelDeadline() async throws {
        let stub = try writeExecutableStub("sleep 30")
        let service = OctCLIService(
            executableURL: stub,
            processTimeout: 300,
            refreshDeadline: 0.5
        )
        let viewModel = UsageViewModel(service: service, configurationStore: ConfigurationStore())

        let recovered = await waitFor(5) {
            !viewModel.isRefreshing
                && viewModel.snapshot.providers.first?.status == .error
        }
        XCTAssertTrue(recovered, "the watchdog must reset isRefreshing and publish an error snapshot")
        XCTAssertNotNil(viewModel.snapshot.providers.first?.message)

        let recoveredMessage = viewModel.snapshot.providers.first?.message
        _ = await waitFor(5.5) { false } // let the background service task land its late result
        XCTAssertFalse(viewModel.isRefreshing, "a late-landing result must not restart the refresh state")
        XCTAssertEqual(
            viewModel.snapshot.providers.first?.message, recoveredMessage,
            "a late-landing result must not clobber the watchdog's error snapshot")
    }

    func testSuccessfulRefreshStillPublishesSnapshot() async throws {
        let stub = try writeExecutableStub(Self.validUsageJSON)
        let service = OctCLIService(executableURL: stub)
        let viewModel = UsageViewModel(service: service, configurationStore: ConfigurationStore())

        let refreshed = await waitFor(5) {
            !viewModel.isRefreshing && !viewModel.snapshot.isPlaceholder
        }
        XCTAssertTrue(refreshed, "the happy path must still land a real snapshot")
        XCTAssertEqual(viewModel.snapshot.providers.first?.name, "codex")
        XCTAssertEqual(viewModel.snapshot.providers.first?.status, .ok)
    }
}
