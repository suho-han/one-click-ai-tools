import XCTest

@testable import OctMenubarApp

private func makeUpdateScript(_ body: String) throws -> URL {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("oct-updater-\(UUID())")
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    let executable = directory.appendingPathComponent("oct")
    try body.write(to: executable, atomically: true, encoding: .utf8)
    try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: executable.path)
    return executable
}

final class ReleaseUpdateTests: XCTestCase {
    @MainActor
    func testAutomaticUpdateCheckToggleHidesReleaseAndPersists() async throws {
        let suite = "oct-release-monitor-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let executable = try makeUpdateScript("""
        #!/bin/sh
        if [ "$*" != "update --check --json" ]; then exit 2; fi
        printf '%s\\n' '{"available":true,"latest":"v0.1.7-beta.2"}'
        """)
        defer { try? FileManager.default.removeItem(at: executable.deletingLastPathComponent()) }
        let service = OctCLIService(executableURL: executable)
        let monitor = ReleaseUpdateMonitor(currentVersion: "0.1.7-beta.1", service: service, defaults: defaults)
        monitor.setAutomaticallyInstallUpdates(false)
        await monitor.checkNow()
        XCTAssertEqual(monitor.availableUpdate?.tag, "v0.1.7-beta.2")

        monitor.setAutomaticallyChecksForUpdates(false)
        XCTAssertFalse(monitor.automaticallyChecksForUpdates)
        XCTAssertNil(monitor.availableUpdate)
        await monitor.checkNow()
        XCTAssertNil(monitor.availableUpdate)

        let reopened = ReleaseUpdateMonitor(currentVersion: "0.1.7-beta.1", service: service, defaults: defaults)
        XCTAssertFalse(reopened.automaticallyChecksForUpdates)
        reopened.setAutomaticallyChecksForUpdates(true)
        await reopened.checkNow()
        XCTAssertEqual(reopened.availableUpdate?.tag, "v0.1.7-beta.2")
    }

    @MainActor
    func testAutomaticInstallDefaultsOnAndPersists() {
        let suite = "oct-auto-install-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }

        let monitor = ReleaseUpdateMonitor(currentVersion: "0.1.8", defaults: defaults)
        XCTAssertTrue(monitor.automaticallyInstallUpdates)

        monitor.setAutomaticallyInstallUpdates(false)
        XCTAssertFalse(monitor.automaticallyInstallUpdates)

        let reopened = ReleaseUpdateMonitor(currentVersion: "0.1.8", defaults: defaults)
        XCTAssertFalse(reopened.automaticallyInstallUpdates)
    }

    @MainActor
    func testAutoInstallAttemptsOncePerTagAndHonorsToggle() {
        let suite = "oct-auto-install-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }

        let monitor = ReleaseUpdateMonitor(currentVersion: "0.1.8", defaults: defaults)
        let update = ReleaseUpdate(tag: "v0.1.9")

        // Opt-in default (Y): the first sight of a tag auto-installs, and the
        // same tag never retries within the session.
        XCTAssertTrue(monitor.shouldAttemptAutoInstall(for: update))
        XCTAssertFalse(monitor.shouldAttemptAutoInstall(for: update))

        // Opted out: no attempt for a different tag either.
        monitor.setAutomaticallyInstallUpdates(false)
        let next = ReleaseUpdate(tag: "v0.1.10")
        XCTAssertFalse(monitor.shouldAttemptAutoInstall(for: next))
    }
}

@MainActor
final class ReleaseUpdateInstallerTests: XCTestCase {
    func testSelfUpdateFailureReturnsCLIDiagnosticWithoutRetrying() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("oct-self-update-\(UUID())")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let executable = directory.appendingPathComponent("oct")
        let attempts = directory.appendingPathComponent("attempts")
        let script = """
        #!/bin/sh
        printf 'attempt\\n' >> '\(attempts.path)'
        if [ "$1" != "update" ] || [ "$2" != "--yes" ] || [ "$#" -ne 2 ]; then
            echo "incorrect update invocation" >&2
            exit 9
        fi
        echo "archive checksum mismatch" >&2
        exit 1
        """
        try script.write(to: executable, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: executable.path)
        let service = OctCLIService(executableURL: executable)

        do {
            _ = try await service.runSelfUpdate()
            XCTFail("a failed oct update must be shown as a failure")
        } catch let error as OctCLIServiceError {
            guard case .nonZeroExit(let status, let stderr) = error else {
                return XCTFail("expected CLI failure, got \(error)")
            }
            XCTAssertEqual(status, 1)
            XCTAssertTrue(stderr.contains("archive checksum mismatch"))
        }
        XCTAssertEqual(try String(contentsOf: attempts, encoding: .utf8), "attempt\n")
    }

    // MARK: performUpdate flow (dialog swapped out — a modal would block tests)

    func testPerformUpdatePresentsFailureAndClearsIsUpdating() async throws {
        let executable = try makeUpdateScript("""
        #!/bin/sh
        echo "archive checksum mismatch" >&2
        exit 1
        """)
        defer { try? FileManager.default.removeItem(at: executable.deletingLastPathComponent()) }
        let installer = ReleaseUpdateInstaller(service: OctCLIService(executableURL: executable))
        var titles: [String] = []
        var buttonTitles: [String] = []
        var restarts = 0
        installer.presentResult = { alert in
            titles.append(alert.messageText)
            buttonTitles = alert.buttons.map(\.title)
            // The lone OK button reports first-button too; a failure must
            // never turn that into a restart.
            return .alertFirstButtonReturn
        }
        installer.restartHelper = { restarts += 1 }

        await installer.performUpdate()

        XCTAssertFalse(installer.isUpdating, "a finished update must release the busy flag")
        XCTAssertEqual(titles, ["oct update failed"])
        XCTAssertFalse(buttonTitles.contains("Restart"), "nothing new to restart into after a failed update")
        XCTAssertEqual(restarts, 0)
    }

    func testPerformUpdatePresentsSuccessWithCLIOutput() async throws {
        let executable = try makeUpdateScript("""
        #!/bin/sh
        echo "oct updated to v0.1.8"
        exit 0
        """)
        defer { try? FileManager.default.removeItem(at: executable.deletingLastPathComponent()) }
        let installer = ReleaseUpdateInstaller(service: OctCLIService(executableURL: executable))
        var titles: [String] = []
        var buttonTitles: [String] = []
        var restarts = 0
        installer.presentResult = { alert in
            titles.append(alert.messageText)
            buttonTitles = alert.buttons.map(\.title)
            return .alertSecondButtonReturn
        }
        installer.restartHelper = { restarts += 1 }

        await installer.performUpdate()

        XCTAssertFalse(installer.isUpdating)
        XCTAssertEqual(titles, ["oct update finished"])
        XCTAssertEqual(buttonTitles, ["Restart", "Later"])
        XCTAssertEqual(restarts, 0, "Later keeps the current helper running")
    }

    func testPerformUpdateRestartsWhenChosen() async throws {
        let executable = try makeUpdateScript("""
        #!/bin/sh
        echo "oct updated to v0.1.8"
        exit 0
        """)
        defer { try? FileManager.default.removeItem(at: executable.deletingLastPathComponent()) }
        let installer = ReleaseUpdateInstaller(service: OctCLIService(executableURL: executable))
        var titles: [String] = []
        var restarts = 0
        installer.presentResult = { alert in
            titles.append(alert.messageText)
            return .alertFirstButtonReturn
        }
        installer.restartHelper = { restarts += 1 }

        await installer.performUpdate()

        XCTAssertEqual(titles, ["oct update finished"])
        XCTAssertEqual(restarts, 1)
    }

    func testPerformUpdateReportsRestartSpawnFailure() async throws {
        let executable = try makeUpdateScript("""
        #!/bin/sh
        exit 0
        """)
        defer { try? FileManager.default.removeItem(at: executable.deletingLastPathComponent()) }
        let installer = ReleaseUpdateInstaller(service: OctCLIService(executableURL: executable))
        var titles: [String] = []
        var details: [String] = []
        installer.presentResult = { alert in
            titles.append(alert.messageText)
            details.append(alert.informativeText)
            return .alertFirstButtonReturn
        }
        installer.restartHelper = {
            throw OctCLIServiceError.launchFailed(
                path: "/bin/zsh",
                underlying: NSError(domain: NSPOSIXErrorDomain, code: Int(ENOENT))
            )
        }

        await installer.performUpdate()

        XCTAssertEqual(titles, ["oct update finished", "Restart failed"])
        XCTAssertTrue(details[1].contains("/bin/zsh"), "the spawn error must reach the user: \(details[1])")
        XCTAssertFalse(installer.isUpdating)
    }
}

