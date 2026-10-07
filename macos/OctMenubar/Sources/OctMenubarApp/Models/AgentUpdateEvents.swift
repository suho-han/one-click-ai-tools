import Foundation

/// One newline-delimited JSON record of `oct agent-update --json`. Mirrors
/// the Go side's flat updateEvent (internal/update/events.go): consumers key
/// off `event` and treat absent fields as zero values, so unknown fields and
/// events from newer CLI versions decode harmlessly.
struct AgentUpdateEvent: Codable, Equatable {
    var event: String
    var total: Int?
    var index: Int?
    var binary: String?
    var name: String?
    var manager: String?
    var versionBefore: String?
    var status: String?
    var versionAfter: String?
    var durationS: Double?
    var error: String?
    var failed: Int?
    // tool_check-only fields (agent-update --check). The bools are optional
    // so "absent" (event kind does not carry the field) stays distinguishable
    // from an explicit false.
    var versionInstalled: String?
    var versionLatest: String?
    var outdated: Bool?
    var latestKnown: Bool?

    enum CodingKeys: String, CodingKey {
        case event, total, index, binary, name, manager, status, error, failed
        case versionBefore = "version_before"
        case versionAfter = "version_after"
        case durationS = "duration_s"
        case versionInstalled = "version_installed"
        case versionLatest = "version_latest"
        case outdated
        case latestKnown = "latest_known"
    }

    /// Tolerant line decoder: non-JSON or foreign lines (progress noise on
    /// stderr-forwarding setups) decode to nil and are dropped.
    static func decode(line: String) -> AgentUpdateEvent? {
        guard let data = line.data(using: .utf8) else { return nil }
        return try? JSONDecoder().decode(AgentUpdateEvent.self, from: data)
    }

    /// tool_done statuses, mirroring the Go eventEmitter's constants.
    static let statusUpdated = "updated"
    static let statusUpToDate = "up_to_date"
    static let statusFailed = "failed"
    static let statusSkipped = "skipped"
    static let statusSkippedNotInstalled = "skipped_not_installed"
    static let statusNpmMissing = "npm_missing"

    /// Event kinds for `agent-update --check --json`.
    static let eventCheckStart = "check_start"
    static let eventToolCheck = "tool_check"
    static let eventCheckDone = "check_done"
}

/// Terminal outcome of one background agent-update run. Per-tool failures
/// travel as tool_done events, so a nonzero exit with a seen run_done event
/// is a normal partial-failure result, not an error.
struct AgentUpdateRunResult: Equatable {
    let exitStatus: Int32
    let stderr: String
}

/// Per-provider chip state while a background agent update runs. Keyed by
/// binaryName in the configuration store — never stored on the draft, whose
/// Equatable conformance drives the unsaved-changes footer. Terminal states
/// persist until the settings window closes (the store clears them on
/// disappear), so the outcome stays readable while the user inspects rows.
enum ToolUpdateState: Equatable {
    case updating
    case updated
    case upToDate
    case failed(String)
    /// The provider's manager needs the Node.js toolchain (npm/pnpm/yarn)
    /// but none of it is on PATH; the row offers a Node.js install link.
    case npmMissing
}

/// Progress through the run's sequential provider loop, rendered next to the
/// run button ("2/5").
struct AgentUpdateProgress: Equatable {
    let current: Int
    let total: Int
}

/// Result of one `agent-update --check` probe for a single provider, keyed
/// by binaryName in the configuration store like ToolUpdateState.
/// `latestKnown == false` marks managers with no query API (native updaters,
/// install scripts): only running their updater can tell whether they are
/// current, so those rows stay button-free — the run-all agent-update is the
/// way to refresh them.
struct ToolVersionCheckState: Equatable {
    var versionInstalled: String?
    var versionLatest: String?
    var latestKnown: Bool
    var outdated: Bool

    /// The update button shows only when the check found a known newer
    /// version; an unknowable latest keeps the row button-free.
    var offersUpdate: Bool {
        outdated
    }

    /// Tooltip for the update button: names the target version when one is
    /// known, otherwise explains that updating doubles as the version check.
    var updateHelp: String? {
        if outdated, let latest = versionLatest, !latest.isEmpty {
            return "Update to v\(latest)"
        }
        if offersUpdate {
            return "Check and update this provider"
        }
        return nil
    }
}

/// Why a background run's providers failed, aggregated for one alert at run
/// end. Each entry mirrors a tool_done(failed) event: the installer's exec
/// error plus the output tail the CLI attaches so the dialog can answer
/// "why did it fail", not just "it failed".
struct AgentUpdateFailureReport: Equatable {
    struct Failure: Equatable {
        let providerName: String
        let error: String
    }

    var failures: [Failure]

    var title: String {
        failures.count == 1 ? "1 provider failed to update" : "\(failures.count) providers failed to update"
    }

    var message: String {
        failures.map { "\($0.providerName): \($0.error)" }.joined(separator: "\n\n")
    }
}
