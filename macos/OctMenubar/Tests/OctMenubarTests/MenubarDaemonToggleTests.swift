import XCTest
@testable import OctMenubarApp

/// The launch-at-login toggle: the store round-trips `oct menubar daemon
/// --json` / enable / disable against a stub oct, including the optimistic
/// update and the failure revert.
@MainActor
final class MenubarDaemonToggleTests: XCTestCase {
    private var directory: URL!
    private var store: ConfigurationStore!

    override func setUpWithError() throws {
        let directory = FileManager.default.temporaryDirectory
            .appendingPathComponent("menubar-daemon-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        self.directory = directory

        let oct = directory.appendingPathComponent("oct")
        let stateFile = directory.appendingPathComponent("daemon-on")
        // `$3` disambiguates the subcommand: --json prints the state file's
        // answer, enable/disable mutate it. The first enable/disable run
        // fails so the revert path is exercised against the same stub.
        let script = """
        #!/bin/sh
        if [ "$3" = "enable" ]; then
            if [ -f "\(directory.path)/fail-first" ]; then rm "\(directory.path)/fail-first"; exit 1; fi
            touch "\(stateFile.path)"
        fi
        if [ "$3" = "disable" ]; then
            rm -f "\(stateFile.path)"
        fi
        if [ "$3" = "--json" ]; then
            if [ -f "\(stateFile.path)" ]; then
                echo '{"enabled":true,"loaded":true,"label":"com.oct.test"}'
            else
                echo '{"enabled":false,"loaded":false,"label":"com.oct.test"}'
            fi
        fi
        """
        try script.write(to: oct, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: oct.path)

        store = ConfigurationStore(service: OctCLIService(executableURL: oct, processTimeout: 10))
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: directory)
    }

    func testLoadParsesDaemonJSON() async throws {
        await store.loadMenubarDaemonState()
        let state = try XCTUnwrap(store.menubarDaemonState)
        XCTAssertFalse(state.enabled)
        XCTAssertFalse(state.loaded)
        XCTAssertEqual(state.label, "com.oct.test")
    }

    func testToggleEnablesAndDisablesThroughTheCLI() async throws {
        await store.loadMenubarDaemonState()

        await store.setMenubarDaemonEnabled(true)
        let enabled = try XCTUnwrap(store.menubarDaemonState)
        XCTAssertTrue(enabled.enabled, "the follow-up read must reflect the enable")
        XCTAssertTrue(enabled.loaded)

        await store.setMenubarDaemonEnabled(false)
        let disabled = try XCTUnwrap(store.menubarDaemonState)
        XCTAssertFalse(disabled.enabled)
        XCTAssertFalse(disabled.loaded)
    }

    func testFailedEnableRevertsAndReportsFeedback() async throws {
        try "fail".write(to: directory.appendingPathComponent("fail-first"), atomically: true, encoding: .utf8)
        await store.loadMenubarDaemonState()
        let before = store.menubarDaemonState

        await store.setMenubarDaemonEnabled(true)

        let state = try XCTUnwrap(store.menubarDaemonState)
        XCTAssertEqual(state, before, "a failed enable must roll the toggle back")
        XCTAssertNotNil(store.feedback, "the failure must surface as feedback")
    }
}
