import Foundation
import SwiftUI

@MainActor
final class UsageViewModel: ObservableObject {
    @Published private(set) var snapshot: UsageSnapshot

    private let service: OctCLIService

    init(service: OctCLIService) {
        self.service = service
        self.snapshot = .placeholder
    }

    func refresh() {
        do {
            let refreshed = try service.fetchUsageSnapshot()
            snapshot = refreshed
        } catch {
            snapshot = .error(message: error.localizedDescription)
        }
    }

    func runAction(_ action: OctMenubarAction) {
        do {
            try service.run(action: action)
        } catch {
            snapshot = .error(message: error.localizedDescription)
        }
    }
}

enum OctMenubarAction {
    case openUsage
    case openMonitor
    case runSessionRefresh
    case runAlertCheck
}
