import Foundation
import Combine
import SwiftUI

/// Single owner of the oct configuration cache. Both the menubar view model
/// and the settings window observe this store, so a save from one side
/// reaches the other (including the refresh timer); external CLI changes are
/// picked up by the explicit reload-on-open policy.
@MainActor
final class ConfigurationStore: ObservableObject {
    @Published private(set) var snapshot: ConfigurationSnapshot?
    @Published var draft: ConfigurationDraft?
    @Published var isLoading = false
    @Published var isSaving = false
    @Published var feedback: SettingsFeedback?
    /// Per-provider progress for the background agent update, keyed by
    /// binaryName. Kept off the draft on purpose: the draft's Equatable
    /// drives the unsaved-changes footer, which must not flicker on every
    /// progress tick.
    @Published private(set) var toolUpdateStates: [String: ToolUpdateState] = [:]
    /// True only while a run-all sweep is in flight — the shape that owns
    /// every enabled provider at once and drives the "n/m" progress counter.
    @Published private(set) var isAgentUpdating = false
    /// Providers with an in-flight row-level (single-provider) run. Kept
    /// separate from isAgentUpdating so one provider's update never disables
    /// the other rows: single runs fly in parallel with each other, but each
    /// shape excludes the other so a provider is never updated twice at once.
    @Published private(set) var singleUpdateBinaries: Set<String> = []
    @Published private(set) var agentUpdateProgress: AgentUpdateProgress?
    /// Latest-version knowledge per provider (from `agent-update --check`),
    /// keyed by binaryName and kept off the draft for the same reason as the
    /// update states: it must never flicker the unsaved-changes footer.
    @Published private(set) var versionChecks: [String: ToolVersionCheckState] = [:]
    @Published private(set) var isCheckingVersions = false
    /// Why the last run's providers failed, presented as one alert when the
    /// run finishes. Nil after dismissal or when the settings window closes.
    @Published private(set) var failureReport: AgentUpdateFailureReport?

    private let service: OctCLIService
    /// Guards against a late-arriving load overwriting a newer draft: only
    /// the most recently started load may publish its result.
    private var loadGeneration = 0
    /// Same guard for version checks: only the newest check may publish.
    private var versionCheckGeneration = 0

    init(service: OctCLIService = OctCLIService(), snapshot: ConfigurationSnapshot? = nil) {
        self.service = service
        self.snapshot = snapshot
    }

    var isRevertAvailable: Bool { snapshot != nil }

    /// True while the edited draft differs from the last loaded snapshot;
    /// drives the pinned save bar at the bottom of the settings window.
    var hasUnsavedChanges: Bool {
        guard let draft, let snapshot else { return false }
        return draft != ConfigurationDraft(snapshot: snapshot)
    }

    /// Silent re-read used when surfaces open (popover, settings window).
    /// Keeps the last known configuration on failure.
    @discardableResult
    func reload() async -> ConfigurationSnapshot? {
        let generation = loadGeneration + 1
        loadGeneration = generation
        do {
            let fresh = try await service.fetchConfigurationSnapshot()
            guard generation == loadGeneration else { return snapshot }
            snapshot = fresh
            return fresh
        } catch {
            guard generation == loadGeneration else { return snapshot }
            feedback = .error(error.localizedDescription)
            return snapshot
        }
    }

    /// User-triggered load in settings: refreshes snapshot and draft with
    /// explicit feedback. Probes tool versions, since the Providers section
    /// shows them; the popover's silent reload skips that cost.
    func loadDraft() async {
        let generation = loadGeneration + 1
        loadGeneration = generation
        isLoading = true
        feedback = nil
        defer { isLoading = false }
        do {
            let fresh = try await service.fetchConfigurationSnapshot(probeVersions: true)
            guard generation == loadGeneration else { return }
            snapshot = fresh
            draft = ConfigurationDraft(snapshot: fresh)
            feedback = .success("Loaded configuration.")
        } catch {
            guard generation == loadGeneration else { return }
            feedback = .error(error.localizedDescription)
        }
    }

    func saveDraft() async {
        guard let pendingDraft = draft, pendingDraft.hasEnabledTool else {
            feedback = .warning("Select at least one provider.")
            return
        }
        isSaving = true
        defer { isSaving = false }
        do {
            try await service.saveConfiguration(pendingDraft.updatePayload())
            // Adopt the freshly saved state so the draft baseline matches;
            // probed again so version labels survive the round-trip.
            let fresh = try await service.fetchConfigurationSnapshot(probeVersions: true)
            snapshot = fresh
            draft = ConfigurationDraft(snapshot: fresh)
            feedback = .success("Saved.")
        } catch {
            feedback = .error(error.localizedDescription)
        }
    }

    func revertDraft() {
        guard let snapshot else { return }
        draft = ConfigurationDraft(snapshot: snapshot)
        feedback = .informational("Reverted to the last loaded configuration.")
    }

    /// Runs `oct agent-update --check --json` and publishes per-provider
    /// latest-version knowledge for the settings rows' update buttons, which
    /// appear only on rows the check proved outdated. A failure (older oct
    /// without `--check`, missing CLI, wedged probe) is deliberately silent:
    /// the buttons just don't appear, and the run-all update button remains
    /// the fallback.
    func checkAgentVersions() async {
        guard !isAgentUpdating, singleUpdateBinaries.isEmpty, !isCheckingVersions else { return }
        let generation = versionCheckGeneration + 1
        versionCheckGeneration = generation
        isCheckingVersions = true
        defer { isCheckingVersions = false }

        let (stream, yieldEvent) = AsyncStream<AgentUpdateEvent>.makeStream(bufferingPolicy: .unbounded)
        let checkTask = Task.detached(priority: .userInitiated) { [service] in
            do {
                let result = try await service.runAgentUpdateCheck { event in
                    yieldEvent.yield(event)
                }
                yieldEvent.finish()
                return result
            } catch {
                yieldEvent.finish()
                throw error
            }
        }

        var sawCheckDone = false
        for await event in stream {
            guard generation == versionCheckGeneration else { return }
            switch event.event {
            case AgentUpdateEvent.eventToolCheck:
                guard let binary = event.binary else { continue }
                let state = ToolVersionCheckState(
                    versionInstalled: event.versionInstalled,
                    versionLatest: event.versionLatest,
                    latestKnown: event.latestKnown ?? false,
                    outdated: event.outdated ?? false
                )
                withAnimation { versionChecks[binary] = state }
            case AgentUpdateEvent.eventCheckDone:
                sawCheckDone = true
            default:
                break
            }
        }
        guard generation == versionCheckGeneration else { return }

        do {
            _ = try await checkTask.value
        } catch {
            withAnimation { versionChecks = [:] }
            return
        }
        if !sawCheckDone {
            // Process exited without a check_done: trust nothing it emitted.
            withAnimation { versionChecks = [:] }
        }
    }

    /// Runs `oct agent-update --json` without a Terminal and animates the
    /// per-provider version chips through Updating… → Updated! → new version
    /// as tool events arrive. `installMissing` follows the user's answer to
    /// the pre-run confirmation dialog: true installs not-installed providers
    /// through their default manager, false skips them. `onlyBinary` narrows
    /// the run to a single provider (the row-level update button), bypassing
    /// the enabled filter — an explicit click is an explicit selection — and
    /// runs in parallel with other rows' single runs. The two run shapes
    /// exclude each other (a sweep owns every enabled provider, so
    /// overlapping one with a row run could update the same provider twice).
    /// The subprocess runs to completion even if the settings window closes:
    /// this task lives on the store, which outlives the window.
    func runAgentUpdateNow(installMissing: Bool, onlyBinary: String? = nil) async {
        guard let pendingDraft = draft else { return }
        if let onlyBinary {
            guard !isAgentUpdating, !singleUpdateBinaries.contains(onlyBinary) else { return }
            guard pendingDraft.tools.contains(where: { $0.binaryName == onlyBinary }) else { return }
            await runProviderUpdate(
                seeding: pendingDraft.tools.filter { $0.binaryName == onlyBinary },
                installMissing: installMissing,
                isRunAll: false
            )
        } else {
            guard !isAgentUpdating, singleUpdateBinaries.isEmpty else { return }
            guard !pendingDraft.tools.filter(\.enabled).isEmpty else {
                feedback = .warning("Select at least one provider.")
                return
            }
            await runProviderUpdate(
                seeding: pendingDraft.tools.filter(\.enabled),
                installMissing: installMissing,
                isRunAll: true
            )
        }
    }

    /// The shared streaming core behind both run shapes: seeds the given
    /// providers' chips, drains one `agent-update --json` process's events,
    /// and finalizes. A sweep replaces the whole chip board (it can only
    /// start when nothing else is in flight); a row run merges into it so a
    /// sibling's in-flight chip survives. Progress counting is sweep-only —
    /// a row run's 1-provider total would just flicker "1/1".
    private func runProviderUpdate(seeding tools: [ConfigTool], installMissing: Bool, isRunAll: Bool) async {
        let seededBinaries = tools.map(\.binaryName)
        withAnimation {
            if isRunAll {
                toolUpdateStates = Dictionary(
                    uniqueKeysWithValues: tools.map { ($0.binaryName, ToolUpdateState.updating) }
                )
            } else {
                for binary in seededBinaries {
                    toolUpdateStates[binary] = .updating
                }
            }
        }
        if isRunAll {
            isAgentUpdating = true
            agentUpdateProgress = nil
        } else {
            singleUpdateBinaries.formUnion(seededBinaries)
        }

        // The service streams events from a pipe-drain thread; an
        // unbounded-buffer stream keeps the delivery order intact while this
        // MainActor loop applies each event to the UI state.
        let (stream, yieldEvent) = AsyncStream<AgentUpdateEvent>.makeStream(bufferingPolicy: .unbounded)
        let runTask = Task.detached(priority: .userInitiated) { [service] in
            do {
                let result = try await service.runAgentUpdate(
                    installMissing: installMissing,
                    only: isRunAll ? nil : seededBinaries
                ) { event in
                    yieldEvent.yield(event)
                }
                yieldEvent.finish()
                return result
            } catch {
                yieldEvent.finish()
                throw error
            }
        }

        var sawRunDone = false
        var updatedCount = 0
        var upToDateCount = 0
        var failedCount = 0
        var npmMissingCount = 0
        var failures: [AgentUpdateFailureReport.Failure] = []
        for await event in stream {
            switch event.event {
            case "tool_start":
                if isRunAll, let total = event.total {
                    agentUpdateProgress = AgentUpdateProgress(current: event.index ?? 0, total: total)
                }
                if let binary = event.binary, toolUpdateStates[binary] != nil {
                    toolUpdateStates[binary] = .updating
                }
            case "tool_done":
                applyToolDone(event, &updatedCount, &upToDateCount, &failedCount, &npmMissingCount, &failures)
            case "run_done":
                sawRunDone = true
            default:
                break
            }
        }

        do {
            let result = try await runTask.value
            finalizeAgentUpdate(
                result: result,
                sawRunDone: sawRunDone,
                updatedCount: updatedCount,
                upToDateCount: upToDateCount,
                failedCount: failedCount,
                npmMissingCount: npmMissingCount,
                failures: failures,
                isRunAll: isRunAll,
                seededBinaries: seededBinaries
            )
        } catch {
            // The run itself failed (launch/timeout): nothing of this run can
            // still be updating — reset its chips instead of hanging on
            // "Updating…". Sibling runs keep theirs.
            withAnimation {
                for binary in seededBinaries where toolUpdateStates[binary] == .updating {
                    toolUpdateStates.removeValue(forKey: binary)
                }
            }
            endRunFlight(isRunAll: isRunAll, seededBinaries: seededBinaries)
            feedback = .error(error.localizedDescription)
        }
    }

    /// Clears whichever in-flight marker this run set. Only one shape can be
    /// airborne per call, but the sweep-shaped branch is skipped on purpose:
    /// a row run must never reset the sweep flag or the progress counter.
    private func endRunFlight(isRunAll: Bool, seededBinaries: [String]) {
        if isRunAll {
            isAgentUpdating = false
            agentUpdateProgress = nil
        } else {
            singleUpdateBinaries.subtract(seededBinaries)
        }
    }

    private func applyToolDone(
        _ event: AgentUpdateEvent,
        _ updatedCount: inout Int,
        _ upToDateCount: inout Int,
        _ failedCount: inout Int,
        _ npmMissingCount: inout Int,
        _ failures: inout [AgentUpdateFailureReport.Failure]
    ) {
        guard let binary = event.binary, toolUpdateStates[binary] != nil else { return }
        let versionAfter = event.versionAfter ?? ""

        switch event.status {
        case AgentUpdateEvent.statusUpdated:
            updatedCount += 1
            withAnimation { toolUpdateStates[binary] = .updated }
            publishVersion(binary, version: versionAfter)
            markVersionCurrent(binary, installedVersion: versionAfter)
        case AgentUpdateEvent.statusUpToDate:
            upToDateCount += 1
            withAnimation { toolUpdateStates[binary] = .upToDate }
            markVersionCurrent(binary, installedVersion: nil)
        case AgentUpdateEvent.statusFailed:
            failedCount += 1
            withAnimation { toolUpdateStates[binary] = .failed(event.error ?? "update failed") }
            failures.append(
                AgentUpdateFailureReport.Failure(
                    providerName: event.name ?? binary,
                    error: event.error ?? "update failed"
                )
            )
        case AgentUpdateEvent.statusNpmMissing:
            npmMissingCount += 1
            withAnimation { toolUpdateStates[binary] = .npmMissing }
        default:
            // Skipped (deadline, missing manager) or not installed: fall back
            // to the plain version chip.
            withAnimation { toolUpdateStates.removeValue(forKey: binary) }
        }
    }

    /// Publishes an updated version into snapshot and draft in lockstep so
    /// hasUnsavedChanges stays exactly as it was before the run.
    private func publishVersion(_ binaryName: String, version: String) {
        guard !version.isEmpty, let snapshot, var draft else { return }
        var updatedSnapshot = snapshot
        guard let snapshotIndex = updatedSnapshot.tools.firstIndex(where: { $0.binaryName == binaryName }) else { return }
        updatedSnapshot.tools[snapshotIndex].version = version
        if let draftIndex = draft.tools.firstIndex(where: { $0.binaryName == binaryName }) {
            draft.tools[draftIndex].version = version
        }
        self.snapshot = updatedSnapshot
        self.draft = draft
    }

    /// After a successful run the check state can move without re-probing:
    /// an updated provider now carries its new version with no known newer
    /// one, and an up-to-date one was never behind. `latestKnown` is left
    /// alone on purpose — for managers without a query API the button must
    /// stay offered (the user's click is the only check there is).
    private func markVersionCurrent(_ binaryName: String, installedVersion: String?) {
        guard var check = versionChecks[binaryName] else { return }
        check.outdated = false
        if let installedVersion, !installedVersion.isEmpty {
            check.versionInstalled = installedVersion
        }
        withAnimation { versionChecks[binaryName] = check }
    }

    private func finalizeAgentUpdate(
        result: AgentUpdateRunResult,
        sawRunDone: Bool,
        updatedCount: Int,
        upToDateCount: Int,
        failedCount: Int,
        npmMissingCount: Int,
        failures: [AgentUpdateFailureReport.Failure],
        isRunAll: Bool,
        seededBinaries: [String]
    ) {
        withAnimation {
            for binary in seededBinaries where toolUpdateStates[binary] == .updating {
                // Providers this run never reached (aborted run) must not hang
                // on "Updating…" forever. Sibling runs' chips are untouched.
                toolUpdateStates.removeValue(forKey: binary)
            }
        }
        endRunFlight(isRunAll: isRunAll, seededBinaries: seededBinaries)

        if !sawRunDone {
            let tail = result.stderr.trimmingCharacters(in: .whitespacesAndNewlines)
            feedback = .error(
                tail.isEmpty
                    ? "Agent update ended unexpectedly (exit status \(result.exitStatus))."
                    : "Agent update ended unexpectedly: \(tail.suffix(200))"
            )
            return
        }
        guard isRunAll else {
            // A row-level run speaks through its own chip (Updated! / Latest /
            // Failed / npm missing) — no summary line, parallel runs would
            // just overwrite each other. Only failures escalate to the alert,
            // which carries the "why" a chip can't.
            if failedCount > 0 {
                failureReport = AgentUpdateFailureReport(failures: failures)
            }
            return
        }
        let summary = "Agent update finished: \(updatedCount) updated, \(upToDateCount) already latest"
        if failedCount > 0 {
            feedback = .warning("\(summary), \(failedCount) failed.")
            // The summary line only counts failures; the alert explains them
            // (installer error + output tail per provider).
            failureReport = AgentUpdateFailureReport(failures: failures)
        } else if npmMissingCount > 0 {
            feedback = .warning("\(summary), \(npmMissingCount) need Node.js/npm.")
        } else if updatedCount > 0 {
            feedback = .success("\(summary).")
        } else if upToDateCount > 0 {
            feedback = .success("All providers are already up to date.")
        } else {
            feedback = .informational("Nothing to update — no installed providers found.")
        }
    }

    /// The settings window closed: the run-outcome chips (Updated!/Latest/
    /// Failed/npm missing) and any undisplayed failure report were for that
    /// viewing session only. Version-check knowledge persists — it stays
    /// valid until the next check.
    func settingsWindowDidClose() {
        guard !toolUpdateStates.isEmpty || failureReport != nil else { return }
        withAnimation {
            toolUpdateStates = [:]
            failureReport = nil
        }
    }

    /// The failure alert was dismissed.
    func dismissFailureReport() {
        failureReport = nil
    }
}
