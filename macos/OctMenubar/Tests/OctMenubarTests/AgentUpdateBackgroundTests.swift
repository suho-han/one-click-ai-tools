import XCTest
@testable import OctMenubarApp

/// Thread-safe event collector: runAgentUpdate's onEvent fires on the stdout
/// drain thread, so a plain array would race the awaiting test task.
private final class EventCollector: @unchecked Sendable {
    private let lock = NSLock()
    private var events: [AgentUpdateEvent] = []

    func append(_ event: AgentUpdateEvent) {
        lock.lock()
        events.append(event)
        lock.unlock()
    }

    var snapshot: [AgentUpdateEvent] {
        lock.lock()
        defer { lock.unlock() }
        return events
    }
}

@MainActor
final class AgentUpdateBackgroundTests: XCTestCase {
    private func writeExecutableStub(_ body: String) throws -> URL {
        let url = FileManager.default.temporaryDirectory
            .appendingPathComponent("oct-menubar-test-\(UUID().uuidString)")
            .appendingPathExtension("sh")
        try "#!/bin/sh\n\(body)\n".write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        addTeardownBlock { try? FileManager.default.removeItem(at: url) }
        return url
    }

    // MARK: - NDJSON decoding

    func testEventDecodesSnakeCaseAndToleratesUnknownFields() throws {
        let line = #"{"event":"tool_done","index":1,"total":3,"binary":"claude","name":"Claude Code","status":"updated","version_after":"2.0.0","duration_s":1.5,"future_field":true}"#
        let event = try XCTUnwrap(AgentUpdateEvent.decode(line: line))
        XCTAssertEqual(event.event, "tool_done")
        XCTAssertEqual(event.binary, "claude")
        XCTAssertEqual(event.versionAfter, "2.0.0")
        XCTAssertEqual(event.durationS, 1.5)
        XCTAssertEqual(event.total, 3)
    }

    func testEventDecodeDropsGarbageLines() {
        XCTAssertNil(AgentUpdateEvent.decode(line: "not json"))
        XCTAssertNil(AgentUpdateEvent.decode(line: ""))
        XCTAssertNil(AgentUpdateEvent.decode(line: #"{"event":42}"#))
    }

    // MARK: - Streaming runner

    func testStreamsEventsInOrderAndReturnsExitStatus() async throws {
        let stub = try writeExecutableStub("""
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_start","binary":"claude","name":"Claude Code","manager":"npm","index":1,"total":1,"version_before":"1.0.0"}'
        sleep 0.1
        echo '{"event":"tool_done","binary":"claude","status":"updated","version_after":"2.0.0","duration_s":1}'
        echo '{"event":"run_done","failed":0}'
        """)
        let service = OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15)
        let collector = EventCollector()

        let result = try await service.runAgentUpdate(timeout: 10) { collector.append($0) }

        XCTAssertEqual(collector.snapshot.map(\.event), ["run_start", "tool_start", "tool_done", "run_done"])
        XCTAssertEqual(collector.snapshot.first(where: { $0.event == "tool_done" })?.versionAfter, "2.0.0")
        XCTAssertEqual(result.exitStatus, 0)
    }

    /// Noise the real CLI never emits — prose lines, a record split across
    /// stdout chunks, and a crash that dies mid-record — must not corrupt the
    /// event stream or hang the runner, and a nonzero exit without run_done
    /// returns (not throws) so the consumer can report the abnormal ending.
    func testToleratesGarbageSplitLinesAndNonZeroExit() async throws {
        let stub = try writeExecutableStub("""
        echo 'brew update noise, not json'
        printf '{"event":"run_start"'
        echo ',"total":2}'
        printf '{"event":"tool_done","binary":"codex","status":"failed","error":"boom"}'
        exit 3
        """)
        let service = OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15)
        let collector = EventCollector()

        let result = try await service.runAgentUpdate(timeout: 10) { collector.append($0) }

        XCTAssertEqual(collector.snapshot.map(\.event), ["run_start", "tool_done"])
        XCTAssertEqual(collector.snapshot[0].total, 2)
        XCTAssertEqual(collector.snapshot[1].error, "boom")
        XCTAssertEqual(result.exitStatus, 3)
    }

    /// A child that traps SIGTERM must still be failed bounded: the
    /// two-stage watchdog SIGKILLs it and the EOF wait has its own deadline.
    func testWatchdogTimeoutThrowsBounded() async throws {
        let stub = try writeExecutableStub("trap '' TERM\nsleep 20")
        let service = OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15)

        let start = Date()
        do {
            _ = try await service.runAgentUpdate(timeout: 0.3, eofGrace: 0.5) { _ in }
            XCTFail("a wedged child must throw, not hang")
        } catch {
            // Expected: the timeout path.
        }
        XCTAssertLessThan(Date().timeIntervalSince(start), 15, "the failure must be deadline-bounded, not a hang")
    }

    func testMissingExecutableThrows() async throws {
        let missing = FileManager.default.temporaryDirectory.appendingPathComponent("oct-missing-\(UUID().uuidString)")
        let service = OctCLIService(executableURL: missing)
        do {
            _ = try await service.runAgentUpdate { _ in }
            XCTFail("a missing oct binary must throw")
        } catch { /* expected */ }
    }

    // MARK: - ConfigurationStore state machine

    func testAgentUpdateArgumentsReflectInstallChoice() {
        XCTAssertEqual(OctCLIService.agentUpdateArguments(installMissing: false), ["agent-update", "--json", "--skip-missing"])
        XCTAssertEqual(OctCLIService.agentUpdateArguments(installMissing: true), ["agent-update", "--json", "--install-missing"])
    }

    func testAgentUpdateArgumentsIncludeOnlyFilter() {
        XCTAssertEqual(
            OctCLIService.agentUpdateArguments(installMissing: false, only: ["claude"]),
            ["agent-update", "--json", "--skip-missing", "--only", "claude"]
        )
        XCTAssertEqual(
            OctCLIService.agentUpdateArguments(installMissing: true, only: []),
            ["agent-update", "--json", "--install-missing"]
        )
    }

    private func makeSnapshot() -> ConfigurationSnapshot {
        ConfigurationSnapshot(
            configFile: "/tmp/oct-config.json",
            menubarTitleMode: .oct,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "Claude Code", binaryName: "claude", enabled: true, version: "1.0.0"),
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true, version: ""),
                ConfigTool(name: "Gemini CLI", binaryName: "agy", enabled: false, version: "9.9.9"),
            ]
        )
    }

    private func updateScript() -> String {
        """
        echo '{"event":"run_start","total":2}'
        echo '{"event":"tool_start","binary":"claude","index":1,"total":2}'
        echo '{"event":"tool_done","binary":"claude","status":"updated","version_after":"2.0.0"}'
        echo '{"event":"tool_start","binary":"codex","index":2,"total":2}'
        echo '{"event":"tool_done","binary":"codex","status":"skipped_not_installed"}'
        echo '{"event":"run_done","failed":0}'
        """
    }

    /// The core contract: outcome chips (Updated! / Latest) persist after the
    /// run instead of settling back — they describe the last run until the
    /// settings window closes — while the new version lands in snapshot AND
    /// draft in lockstep so the unsaved-changes footer never flickers; a
    /// skipped not-installed provider falls back to its plain chip; disabled
    /// providers are untouched.
    func testRunAgentUpdateAnimatesChipsAndSyncsVersions() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub(updateScript())
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: false)

        XCTAssertEqual(store.toolUpdateStates["claude"], .updated, "the Updated! chip persists after the run")
        XCTAssertNil(store.toolUpdateStates["codex"], "a skipped provider falls back to its plain chip")
        XCTAssertNil(store.toolUpdateStates["agy"], "disabled providers never enter the update flow")
        XCTAssertFalse(store.isAgentUpdating)
        XCTAssertNil(store.agentUpdateProgress)
        XCTAssertEqual(store.feedback?.message, "Agent update finished: 1 updated, 0 already latest.")

        XCTAssertEqual(store.snapshot?.tools.first { $0.binaryName == "claude" }?.version, "2.0.0")
        XCTAssertEqual(store.draft?.tools.first { $0.binaryName == "claude" }?.version, "2.0.0")
        XCTAssertFalse(store.hasUnsavedChanges, "version sync must land on snapshot and draft in lockstep")

        store.settingsWindowDidClose()
        XCTAssertTrue(store.toolUpdateStates.isEmpty, "closing the settings window clears the run-outcome chips")
    }

    /// The confirmation-dialog path: with installMissing the not-installed
    /// provider (version "") is installed through its default manager and
    /// the fresh version lands on the chip, draft, and snapshot in lockstep.
    func testInstallMissingInstallsProviderAndSyncsVersion() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub("""
        echo '{"event":"run_start","total":2}'
        echo '{"event":"tool_done","binary":"claude","status":"up_to_date","version_after":"1.0.0"}'
        echo '{"event":"tool_done","binary":"codex","status":"updated","version_after":"1.2.3"}'
        echo '{"event":"run_done","failed":0}'
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: true)

        XCTAssertEqual(store.draft?.tools.first { $0.binaryName == "codex" }?.version, "1.2.3")
        XCTAssertEqual(store.snapshot?.tools.first { $0.binaryName == "codex" }?.version, "1.2.3")
        XCTAssertFalse(store.hasUnsavedChanges)
        if case .success = store.feedback?.kind {} else {
            XCTFail("an all-good install run must succeed, got \(String(describing: store.feedback))")
        }
    }

    /// A provider whose manager needs npm with npm absent gets the
    /// persistent npm-missing chip (the row shows a Node.js install link)
    /// and the summary calls it out.
    func testNpmMissingStatePersistsWithNodeInstallHint() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub("""
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_done","binary":"claude","status":"npm_missing","error":"npm not found in PATH. Claude Code installs via npm and has no standalone installer"}'
        echo '{"event":"run_done","failed":1}'
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: true)

        XCTAssertEqual(store.toolUpdateStates["claude"], .npmMissing)
        XCTAssertEqual(store.snapshot?.tools.first { $0.binaryName == "claude" }?.version, "1.0.0")
        XCTAssertTrue(
            store.feedback?.message.contains("need Node.js/npm") == true,
            "got \(String(describing: store.feedback))"
        )
        if case .warning = store.feedback?.kind {} else {
            XCTFail("npm-missing outcomes must warn, got \(String(describing: store.feedback))")
        }
    }

    func testFailedUpdateKeepsVersionAndPersistsFailureChip() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub("""
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_done","binary":"claude","name":"Claude Code","status":"failed","error":"brew upgrade died"}'
        echo '{"event":"run_done","failed":1}'
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: false)

        XCTAssertEqual(store.toolUpdateStates["claude"], .failed("brew upgrade died"))
        XCTAssertEqual(store.snapshot?.tools.first { $0.binaryName == "claude" }?.version, "1.0.0", "a failed provider keeps its old version")
        XCTAssertFalse(store.hasUnsavedChanges)
        if case .warning = store.feedback?.kind {} else {
            XCTFail("a run with failures must warn, got \(String(describing: store.feedback))")
        }

        // The failure alert explains the "why", not just the count.
        XCTAssertEqual(
            store.failureReport,
            AgentUpdateFailureReport(failures: [.init(providerName: "Claude Code", error: "brew upgrade died")])
        )
        XCTAssertEqual(store.failureReport?.title, "1 provider failed to update")
        XCTAssertEqual(store.failureReport?.message, "Claude Code: brew upgrade died")

        store.dismissFailureReport()
        XCTAssertNil(store.failureReport, "dismissing the alert clears the report")
    }

    /// Multiple failures aggregate into one report with one entry per
    /// provider, keeping the installer's own error text.
    func testFailureReportAggregatesMultipleFailures() async throws {
        let snapshot = makeSnapshot()
        // printf '%s' keeps the JSON's \n escape literal — sh echo would
        // (depending on the shell) turn it into a real newline and split the
        // record, which the decoder must drop.
        let stub = try writeExecutableStub("""
        echo '{"event":"run_start","total":2}'
        echo '{"event":"tool_done","binary":"claude","name":"Claude Code","status":"failed","error":"brew upgrade died"}'
        printf '%s\\n' '{"event":"tool_done","binary":"codex","name":"OpenAI Codex","status":"failed","error":"exit status 1\\nnpm error 404 not found"}'
        echo '{"event":"run_done","failed":2}'
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: false)

        XCTAssertEqual(store.failureReport?.failures.map(\.providerName), ["Claude Code", "OpenAI Codex"])
        XCTAssertEqual(store.failureReport?.title, "2 providers failed to update")
        XCTAssertEqual(store.failureReport?.failures.last?.error, "exit status 1\nnpm error 404 not found")
        XCTAssertTrue(store.failureReport?.message.contains("\n\n") == true, "entries are separated by blank lines")
    }

    /// Closing the settings window ends the viewing session: run-outcome
    /// chips and any undisplayed failure report clear, but version-check
    /// knowledge stays valid for the update buttons.
    func testSettingsWindowCloseClearsRunOutcomeButKeepsChecks() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub("""
        if [ "$2" = "--check" ]; then
        echo '{"event":"check_start","total":1}'
        echo '{"event":"tool_check","binary":"claude","manager":"npm","version_installed":"1.0.0","version_latest":"2.0.0","outdated":true,"latest_known":true}'
        echo '{"event":"check_done"}'
        exit 0
        fi
        echo '{"event":"run_start","total":2}'
        echo '{"event":"tool_done","binary":"claude","status":"updated","version_after":"2.0.0"}'
        echo '{"event":"tool_done","binary":"codex","status":"failed","error":"boom"}'
        echo '{"event":"run_done","failed":1}'
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.checkAgentVersions()
        await store.runAgentUpdateNow(installMissing: false)

        XCTAssertFalse(store.toolUpdateStates.isEmpty)
        XCTAssertNotNil(store.failureReport)

        store.settingsWindowDidClose()

        XCTAssertTrue(store.toolUpdateStates.isEmpty, "run-outcome chips clear on window close")
        XCTAssertNil(store.failureReport, "an undisplayed failure report clears on window close")
        XCTAssertNotNil(store.versionChecks["claude"], "version-check knowledge survives the window close")
    }

    func testRunWithoutRunDoneReportsUnexpectedEnding() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub("echo 'crash'; exit 9")
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: false)

        XCTAssertFalse(store.isAgentUpdating)
        XCTAssertTrue(
            store.feedback?.message.contains("ended unexpectedly") == true,
            "got \(String(describing: store.feedback))"
        )
    }

    func testRunRefusesWhenNoProviderEnabled() async throws {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/oct-config.json",
            menubarTitleMode: .oct,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [ConfigTool(name: "Claude Code", binaryName: "claude", enabled: false, version: "1.0.0")]
        )
        let stub = try writeExecutableStub("echo 'must not run'")
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: false)

        XCTAssertFalse(store.isAgentUpdating)
        XCTAssertTrue(store.toolUpdateStates.isEmpty)
    }

    // MARK: - Version check (agent-update --check)

    private func makeCheckStub(events: String, alsoWriteArgsTo argsFile: URL? = nil) throws -> URL {
        let argsLine: String
        if let argsFile {
            argsLine = "printf '%s\\n' \"$@\" > '\(argsFile.path)'\n"
        } else {
            argsLine = ""
        }
        return try writeExecutableStub("""
        \(argsLine)if [ "$2" = "--check" ]; then
        \(events)
        exit 0
        fi
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_done","binary":"claude","status":"updated","version_after":"2.0.0"}'
        echo '{"event":"run_done","failed":0}'
        """)
    }

    /// The check pipeline end to end: tool_check events become per-provider
    /// check states — an outdated npm provider offers "Update to v…", while a
    /// manager without a query API (claude-native) offers no button at all.
    func testCheckAgentVersionsPublishesStates() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub("""
        if [ "$2" = "--check" ]; then
        echo '{"event":"check_start","total":3}'
        echo '{"event":"tool_check","binary":"claude","manager":"npm","version_installed":"1.0.0","version_latest":"2.0.0","outdated":true,"latest_known":true}'
        echo '{"event":"tool_check","binary":"agy","manager":"claude-native","version_installed":"9.9.9","outdated":false,"latest_known":false}'
        echo '{"event":"tool_check","binary":"codex","manager":"brew","version_installed":"","outdated":false,"latest_known":true}'
        echo '{"event":"check_done"}'
        exit 0
        fi
        echo 'unexpected run'
        exit 1
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.checkAgentVersions()

        XCTAssertFalse(store.isCheckingVersions)
        XCTAssertEqual(
            store.versionChecks["claude"],
            ToolVersionCheckState(versionInstalled: "1.0.0", versionLatest: "2.0.0", latestKnown: true, outdated: true)
        )
        XCTAssertEqual(store.versionChecks["claude"]?.updateHelp, "Update to v2.0.0")
        XCTAssertEqual(
            store.versionChecks["agy"],
            ToolVersionCheckState(versionInstalled: "9.9.9", versionLatest: nil, latestKnown: false, outdated: false)
        )
        // Latest unknowable and nothing provably newer: the row stays
        // button-free; the run-all sweep is the way to refresh it.
        XCTAssertFalse(store.versionChecks["agy"]?.offersUpdate ?? true)
        XCTAssertNil(store.versionChecks["agy"]?.updateHelp)
        // Up to date with a queryable manager: no button at all.
        XCTAssertEqual(
            store.versionChecks["codex"],
            ToolVersionCheckState(versionInstalled: "", versionLatest: nil, latestKnown: true, outdated: false)
        )
        XCTAssertFalse(store.versionChecks["codex"]?.offersUpdate ?? true)
    }

    /// A check that never emits check_done (older oct without --check exits
    /// with an unknown-flag error, or the CLI dies mid-run) must leave no
    /// stale buttons behind.
    func testCheckAgentVersionsClearsStatesWhenRunFails() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub("""
        echo '{"event":"tool_check","binary":"claude","outdated":true,"latest_known":true}'
        echo 'Error: unknown flag: --check' >&2
        exit 1
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.checkAgentVersions()

        XCTAssertTrue(store.versionChecks.isEmpty, "a failed check must not leave stale update buttons")
    }

    /// The full row-level flow: the check flags claude outdated, the user
    /// clicks the provider's update button, and the successful run clears the
    /// outdated flag without a re-probe (and syncs the new version).
    func testSingleProviderRunClearsOutdatedFlagAndSyncsVersion() async throws {
        let snapshot = makeSnapshot()
        let argsFile = FileManager.default.temporaryDirectory
            .appendingPathComponent("oct-args-\(UUID().uuidString)")
        addTeardownBlock { try? FileManager.default.removeItem(at: argsFile) }
        let stub = try makeCheckStub(
            events: """
            echo '{"event":"check_start","total":1}'
            echo '{"event":"tool_check","binary":"claude","manager":"npm","version_installed":"1.0.0","version_latest":"2.0.0","outdated":true,"latest_known":true}'
            echo '{"event":"check_done"}'
            """,
            alsoWriteArgsTo: argsFile
        )
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.checkAgentVersions()
        XCTAssertEqual(store.versionChecks["claude"]?.outdated, true)

        await store.runAgentUpdateNow(installMissing: false, onlyBinary: "claude")

        let args = (try? String(contentsOf: argsFile, encoding: .utf8)) ?? ""
        XCTAssertTrue(args.contains("--only"), "the single-provider run must pass --only, got: \(args)")
        XCTAssertTrue(args.contains("claude"))
        XCTAssertEqual(store.toolUpdateStates["claude"], .updated, "the clicked provider's chip animates")
        XCTAssertNil(store.toolUpdateStates["codex"], "other providers are never seeded")
        XCTAssertNil(store.toolUpdateStates["agy"], "not even enabled ones outside the selection")
        XCTAssertFalse(store.versionChecks["claude"]?.outdated ?? true, "a successful run clears the outdated flag")
        XCTAssertEqual(store.versionChecks["claude"]?.versionInstalled, "2.0.0")
        XCTAssertEqual(store.snapshot?.tools.first { $0.binaryName == "claude" }?.version, "2.0.0")
        XCTAssertFalse(store.hasUnsavedChanges)
        XCTAssertFalse(store.isAgentUpdating)
    }

    /// The row-level button bypasses the enabled filter — an explicit click
    /// on a disabled provider's row is an explicit selection — but an unknown
    /// binaryName is refused without spawning anything.
    func testSingleProviderRunAcceptsDisabledAndRefusesUnknown() async throws {
        let snapshot = makeSnapshot()
        let argsFile = FileManager.default.temporaryDirectory
            .appendingPathComponent("oct-args-\(UUID().uuidString)")
        addTeardownBlock { try? FileManager.default.removeItem(at: argsFile) }
        let stub = try writeExecutableStub("""
        printf '%s\\n' \"$@\" > '\(argsFile.path)'
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_done","binary":"agy","status":"updated","version_after":"10.0.0"}'
        echo '{"event":"run_done","failed":0}'
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: false, onlyBinary: "agy")

        let args = (try? String(contentsOf: argsFile, encoding: .utf8)) ?? ""
        XCTAssertTrue(args.contains("agy"))
        XCTAssertEqual(store.toolUpdateStates["agy"], .updated)
        XCTAssertEqual(store.draft?.tools.first { $0.binaryName == "agy" }?.version, "10.0.0")

        let statesBefore = store.toolUpdateStates
        await store.runAgentUpdateNow(installMissing: false, onlyBinary: "no-such-binary")
        XCTAssertEqual(store.toolUpdateStates, statesBefore, "an unknown binaryName must not start a run")
    }

    /// The headline parallelism contract: while one provider's row-level run
    /// is airborne, another row's click starts its own run — each animating
    /// only its own chip — and a later run must not wipe the first one's
    /// in-flight chip. isAgentUpdating stays false throughout, which is what
    /// keeps the other rows' update buttons enabled in the UI.
    func testParallelSingleProviderRunsUpdateIndependently() async throws {
        let snapshot = makeSnapshot()
        // $4 is "--only" and $5 the binary for row runs (agent-update --json
        // --skip-missing --only <bin>); claude's branch is slow so the test
        // can observe agy finishing while claude is still in flight.
        let stub = try writeExecutableStub("""
        if [ "$5" = "claude" ]; then
        sleep 0.5
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_done","binary":"claude","status":"updated","version_after":"2.0.0"}'
        echo '{"event":"run_done","failed":0}'
        else
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_done","binary":"agy","status":"updated","version_after":"10.0.0"}'
        echo '{"event":"run_done","failed":0}'
        fi
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        let claudeRun = Task { await store.runAgentUpdateNow(installMissing: false, onlyBinary: "claude") }
        for _ in 0..<200 where store.singleUpdateBinaries.isEmpty {
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        XCTAssertEqual(store.singleUpdateBinaries, ["claude"], "claude's run must be airborne before agy's starts")

        await store.runAgentUpdateNow(installMissing: false, onlyBinary: "agy")

        XCTAssertEqual(store.toolUpdateStates["agy"], .updated)
        XCTAssertEqual(
            store.toolUpdateStates["claude"],
            .updating,
            "agy's run must merge into the chip board, not wipe claude's in-flight chip"
        )
        XCTAssertFalse(store.isAgentUpdating, "row runs never raise the sweep flag — sibling buttons stay enabled")

        await claudeRun.value

        XCTAssertEqual(store.toolUpdateStates["claude"], .updated)
        XCTAssertTrue(store.singleUpdateBinaries.isEmpty, "both runs released their slots")
        XCTAssertFalse(store.isAgentUpdating)
    }

    /// The two run shapes exclude each other: a row click mid-sweep is
    /// refused (a sweep owns every enabled provider), and a sweep refuses to
    /// start while any row run is airborne. Refusals leave the other run's
    /// chips and flags intact.
    func testRunAllAndRowRunExcludeEachOther() async throws {
        let snapshot = makeSnapshot()
        // $4 is "--only" only for row runs; the sweep (no --only) takes the
        // else branch. Both branches sleep so the test can interleave.
        let stub = try writeExecutableStub("""
        if [ "$4" = "--only" ]; then
        sleep 0.4
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_done","binary":"agy","status":"updated","version_after":"10.0.0"}'
        echo '{"event":"run_done","failed":0}'
        else
        sleep 0.4
        echo '{"event":"run_start","total":2}'
        echo '{"event":"tool_done","binary":"claude","status":"updated","version_after":"2.0.0"}'
        echo '{"event":"run_done","failed":0}'
        fi
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        let sweep = Task { await store.runAgentUpdateNow(installMissing: false) }
        for _ in 0..<200 where !store.isAgentUpdating {
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        XCTAssertTrue(store.isAgentUpdating)

        await store.runAgentUpdateNow(installMissing: false, onlyBinary: "agy")

        XCTAssertTrue(store.singleUpdateBinaries.isEmpty, "a row click mid-sweep is refused")
        XCTAssertNil(store.toolUpdateStates["agy"])

        await sweep.value

        XCTAssertEqual(store.toolUpdateStates["claude"], .updated)
        XCTAssertFalse(store.isAgentUpdating)

        let rowRun = Task { await store.runAgentUpdateNow(installMissing: false, onlyBinary: "agy") }
        for _ in 0..<200 where store.singleUpdateBinaries.isEmpty {
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        XCTAssertEqual(store.singleUpdateBinaries, ["agy"])

        await store.runAgentUpdateNow(installMissing: false)

        XCTAssertFalse(store.isAgentUpdating, "a sweep during a row run is refused")
        XCTAssertEqual(store.toolUpdateStates["claude"], .updated, "the refused sweep must not reset the chip board")

        await rowRun.value

        XCTAssertEqual(store.toolUpdateStates["agy"], .updated)
        XCTAssertTrue(store.singleUpdateBinaries.isEmpty)
    }

    /// A failed row-level run still escalates to the failure alert (the chip
    /// alone can't explain why) and releases its singleUpdateBinaries slot —
    /// but leaves the summary feedback line alone: parallel row runs would
    /// overwrite each other, and the chip carries the outcome.
    func testFailedRowRunRaisesFailureAlertAndReleasesSlot() async throws {
        let snapshot = makeSnapshot()
        let stub = try writeExecutableStub("""
        echo '{"event":"run_start","total":1}'
        echo '{"event":"tool_done","binary":"claude","name":"Claude Code","status":"failed","error":"brew upgrade died"}'
        echo '{"event":"run_done","failed":1}'
        """)
        let store = ConfigurationStore(
            service: OctCLIService(executableURL: stub, processTimeout: 10, refreshDeadline: 15),
            snapshot: snapshot
        )
        store.draft = ConfigurationDraft(snapshot: snapshot)

        await store.runAgentUpdateNow(installMissing: false, onlyBinary: "claude")

        XCTAssertEqual(store.toolUpdateStates["claude"], .failed("brew upgrade died"))
        XCTAssertEqual(store.failureReport?.failures.map(\.providerName), ["Claude Code"])
        XCTAssertTrue(store.singleUpdateBinaries.isEmpty)
        XCTAssertFalse(store.isAgentUpdating)
        XCTAssertNil(store.feedback, "row runs leave the summary line to the sweep")
    }
}
