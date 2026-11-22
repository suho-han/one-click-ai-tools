import Combine
import Foundation
import os

/// A newer GitHub release of oct, ready to surface in the menubar.
struct ReleaseUpdate: Equatable {
    let tag: String
}

extension ReleaseUpdateMonitor {
    static let logger = Logger(subsystem: "app.oct.menubar", category: "release-update")
}

/// Checks for updates through the CLI every five minutes. Failures keep the
/// last known state rather than hiding a previously available update.
@MainActor
final class ReleaseUpdateMonitor: ObservableObject {
    static let shared = ReleaseUpdateMonitor()
    static let repo = "suho-han/one-click-ai-tools"
    static let pollInterval: TimeInterval = 5 * 60

    @Published private(set) var availableUpdate: ReleaseUpdate?
    let installer: ReleaseUpdateInstaller
    @Published private(set) var automaticallyChecksForUpdates: Bool
    @Published private(set) var automaticallyInstallUpdates: Bool

    private var timer: Timer?
    private var hasStarted = false
    private var checkGeneration = 0
    /// One automatic install attempt per tag per session, so a failed
    /// `oct update` does not retry on every poll interval.
    private var autoAttemptedTags: Set<String> = []
    private let checksReleaseBuild: Bool
    private let service: OctCLIService
    private let defaults: UserDefaults

    init(
        currentVersion: String = MenubarBuildVersion.version,
        service: OctCLIService = OctCLIService(),
        defaults: UserDefaults = .standard
    ) {
        self.checksReleaseBuild = currentVersion != "dev"
        self.service = service
        self.defaults = defaults
        self.installer = ReleaseUpdateInstaller(service: service)
        self.automaticallyChecksForUpdates =
            defaults.object(forKey: MenubarPreferences.automaticallyChecksForUpdatesKey) as? Bool ?? true
        self.automaticallyInstallUpdates =
            defaults.object(forKey: MenubarPreferences.automaticallyInstallUpdatesKey) as? Bool ?? true
    }

    func start() {
        guard !hasStarted else { return }
        hasStarted = true
        beginPollingIfEnabled()
    }

    func setAutomaticallyChecksForUpdates(_ enabled: Bool) {
        guard enabled != automaticallyChecksForUpdates else { return }
        defaults.set(enabled, forKey: MenubarPreferences.automaticallyChecksForUpdatesKey)
        automaticallyChecksForUpdates = enabled
        if enabled {
            beginPollingIfEnabled()
        } else {
            timer?.invalidate()
            timer = nil
            checkGeneration += 1
            availableUpdate = nil
        }
    }

    func setAutomaticallyInstallUpdates(_ enabled: Bool) {
        guard enabled != automaticallyInstallUpdates else { return }
        defaults.set(enabled, forKey: MenubarPreferences.automaticallyInstallUpdatesKey)
        automaticallyInstallUpdates = enabled
        // A pending release updates right away instead of waiting for the
        // next poll interval.
        if enabled, let update = availableUpdate {
            maybeAutoInstall(update)
        }
    }

    /// Starts at most one automatic `oct update` per release tag per session
    /// when the user opted in; otherwise the update waits for its prompt.
    func maybeAutoInstall(_ update: ReleaseUpdate) {
        guard shouldAttemptAutoInstall(for: update) else { return }
        installer.requestAutomaticUpdate()
    }

    /// The decision half of maybeAutoInstall, separated so tests can exercise
    /// the once-per-tag bookkeeping without reaching the updater.
    func shouldAttemptAutoInstall(for update: ReleaseUpdate) -> Bool {
        guard automaticallyInstallUpdates, !autoAttemptedTags.contains(update.tag) else { return false }
        autoAttemptedTags.insert(update.tag)
        return true
    }

    private func beginPollingIfEnabled() {
        guard hasStarted, automaticallyChecksForUpdates, timer == nil else { return }
        Task { await checkNow() }
        timer = Timer.scheduledTimer(withTimeInterval: Self.pollInterval, repeats: true) { [weak self] _ in
            Task { await self?.checkNow() }
        }
    }

    func checkNow() async {
        guard checksReleaseBuild, automaticallyChecksForUpdates else { return }
        let generation = checkGeneration
        do {
            let update = try await service.checkSelfUpdate()
            guard automaticallyChecksForUpdates, generation == checkGeneration else { return }
            availableUpdate = update
            if let update = availableUpdate {
                maybeAutoInstall(update)
            }
        } catch {
            // Lookup and subprocess failures leave the previous result visible.
            Self.logger.notice("Release check failed: \(error.localizedDescription, privacy: .public)")
        }
    }
}
