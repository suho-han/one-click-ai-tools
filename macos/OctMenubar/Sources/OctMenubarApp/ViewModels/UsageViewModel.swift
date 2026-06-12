import Foundation
import SwiftUI

@MainActor
final class UsageViewModel: ObservableObject {
    @Published private(set) var snapshot: UsageSnapshot
    @Published private(set) var isRefreshing = false

    private let service: OctCLIService
    private var refreshTimer: Timer?

    init(service: OctCLIService) {
        self.service = service
        self.snapshot = .placeholder
        scheduleRefreshTimer()
        refresh()
    }

    func refresh(now: Date = Date()) {
        guard !isRefreshing else { return }
        isRefreshing = true
        defer { isRefreshing = false }

        do {
            snapshot = try service.fetchUsageSnapshot(now: now)
        } catch {
            snapshot = .error(message: error.localizedDescription, refreshInterval: service.refreshInterval)
        }
    }

    func runAction(_ action: OctMenubarAction) {
        do {
            try service.run(action: action)
        } catch {
            snapshot = .error(message: error.localizedDescription, refreshInterval: service.refreshInterval)
        }
    }

    private func scheduleRefreshTimer() {
        refreshTimer?.invalidate()
        refreshTimer = Timer.scheduledTimer(withTimeInterval: service.refreshInterval, repeats: true) { [weak self] _ in
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
