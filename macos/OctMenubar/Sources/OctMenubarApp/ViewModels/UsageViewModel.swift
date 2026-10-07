import AppKit
import Combine
import Foundation
import SwiftUI

@MainActor
final class UsageViewModel: ObservableObject {
    @Published private(set) var snapshot: UsageSnapshot
    @Published private(set) var isRefreshing = false

    private let service: OctCLIService
    private let configurationStore: ConfigurationStore
    private var refreshTimer: Timer?
    private var configurationCancellable: AnyCancellable?
    /// Guards the refresh completion path: only the newest generation may
    /// publish, so a late result from a wedged refresh (watchdog already
    /// fired) cannot resurrect stale data or clobber a newer refresh.
    private var generation = 0
    private var watchdogTask: Task<Void, Never>?

    init(service: OctCLIService, configurationStore: ConfigurationStore) {
        self.service = service
        self.configurationStore = configurationStore
        self.snapshot = .placeholder
        // A configuration save (settings or external reload) reschedules the
        // refresh timer so the new interval applies immediately. Subscribing
        // to $snapshot (fires after the value lands) keeps the reschedule
        // from reading the stale interval via objectWillChange.
        configurationCancellable = configurationStore.$snapshot
            .receive(on: RunLoop.main)
            .sink { [weak self] _ in
                self?.scheduleRefreshTimer()
            }
        scheduleRefreshTimer()
        refresh()
    }

    func refresh(now: Date = Date()) {
        guard !isRefreshing else { return }
        isRefreshing = true
        generation += 1
        let refreshGeneration = generation
        let configuration = configurationStore.snapshot
        let refreshDeadline = service.refreshDeadline
        watchdogTask?.cancel()
        // Completion deadline: runProcess bounds its subprocess waits, but
        // any single hang must never wedge the menubar (a stuck refresh once
        // left isRefreshing true forever and every later refresh was dropped
        // by the guard below). The watchdog fails this refresh and re-arms
        // the timer-driven cycle; a late-landing result is discarded by the
        // generation guard.
        watchdogTask = Task { [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(refreshDeadline * 1_000_000_000))
            guard !Task.isCancelled else { return }
            self?.finishRefresh(
                generation: refreshGeneration,
                outcome: .failure("usage refresh did not finish within \(Int(refreshDeadline))s; it will retry on the next cycle")
            )
        }
        Task.detached(priority: .userInitiated) { [service] in
            do {
                let refreshed = try await service.fetchUsageSnapshot(configuration: configuration, now: now)
                await MainActor.run { [weak self] in
                    self?.finishRefresh(generation: refreshGeneration, outcome: .success(refreshed))
                }
            } catch {
                await MainActor.run { [weak self] in
                    self?.finishRefresh(generation: refreshGeneration, outcome: .failure(error.localizedDescription))
                }
            }
        }
    }

    private enum RefreshOutcome {
        case success(UsageSnapshot)
        case failure(String)
    }

    /// Single completion gate for refreshes: the newest generation wins,
    /// publishing always cancels the watchdog and resets isRefreshing.
    private func finishRefresh(generation refreshGeneration: Int, outcome: RefreshOutcome) {
        guard isRefreshing, refreshGeneration == generation else { return }
        generation += 1
        watchdogTask?.cancel()
        watchdogTask = nil
        isRefreshing = false
        switch outcome {
        case .success(let fresh):
            snapshot = fresh
        case .failure(let message):
            snapshot = .error(message: message, refreshInterval: currentRefreshInterval())
        }
    }

    func runAction(_ action: OctMenubarAction) {
        // Process I/O stays off the main thread; a failure surfaces as
        // action feedback without wiping the displayed usage data.
        Task {
            do {
                try await service.run(action: action)
            } catch {
                snapshot = .error(message: error.localizedDescription, refreshInterval: currentRefreshInterval())
            }
        }
    }

    /// Stops this helper and starts a fresh detached one. The replacement is
    /// already queued in a detached shell when this returns, so terminating
    /// right away cannot leave the menubar down; a spawn failure surfaces as
    /// action feedback instead of quitting.
    func restartHelper() {
        do {
            try service.restartHelperDetached()
        } catch {
            snapshot = .error(message: error.localizedDescription, refreshInterval: currentRefreshInterval())
            return
        }
        NSApp.terminate(nil)
    }

    private func currentRefreshInterval() -> TimeInterval {
        configurationStore.snapshot?.refreshInterval ?? service.refreshInterval
    }

    private func scheduleRefreshTimer() {
        refreshTimer?.invalidate()
        refreshTimer = Timer.scheduledTimer(withTimeInterval: currentRefreshInterval(), repeats: true) { [weak self] _ in
            Task { @MainActor in
                self?.refresh()
            }
        }
        if let refreshTimer {
            RunLoop.main.add(refreshTimer, forMode: .common)
        }
    }
}

enum OctMenubarAction {
    case openUsage
    case openMonitor
    case runSessionRefresh
    case runAlertCheck
    case runAgentUpdate
}
