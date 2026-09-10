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
        let configuration = configurationStore.snapshot
        Task.detached(priority: .userInitiated) { [service] in
            do {
                let refreshed = try await service.fetchUsageSnapshot(configuration: configuration, now: now)
                await MainActor.run {
                    self.snapshot = refreshed
                    self.isRefreshing = false
                }
            } catch {
                await MainActor.run {
                    let interval = configuration?.refreshInterval ?? service.refreshInterval
                    self.snapshot = .error(message: error.localizedDescription, refreshInterval: interval)
                    self.isRefreshing = false
                }
            }
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
}
