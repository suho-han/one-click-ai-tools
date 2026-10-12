import Combine
import Foundation

@MainActor
final class FeedbackStore: ObservableObject {
    @Published var message = "" {
        didSet { if message != oldValue { status = nil } }
    }
    @Published private(set) var isSubmitting = false
    @Published private(set) var status: SettingsFeedback?
    private let service: FeedbackService

    init(service: FeedbackService) { self.service = service }

    var isAvailable: Bool { service.isAvailable }
    var messageByteCount: Int { message.utf8.count }
    var canSubmit: Bool { isAvailable && !isSubmitting && FeedbackService.validMessage(message) }

    func submit() async {
        guard canSubmit else { return }
        isSubmitting = true
        status = nil
        let submittedMessage = message
        defer { isSubmitting = false }
        do {
            try await service.submit(message: submittedMessage)
            message = ""
            status = .success("Feedback sent. Thank you!")
        } catch {
            status = .error(error.localizedDescription)
        }
    }
}
