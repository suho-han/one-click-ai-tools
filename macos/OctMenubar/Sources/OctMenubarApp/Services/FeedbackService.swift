import Foundation

private final class FeedbackRedirectDelegate: NSObject, URLSessionTaskDelegate {
    func urlSession(_ session: URLSession, task: URLSessionTask,
                    willPerformHTTPRedirection response: HTTPURLResponse,
                    newRequest request: URLRequest,
                    completionHandler: @escaping (URLRequest?) -> Void) {
        completionHandler(nil)
    }
}

enum FeedbackSubmissionError: LocalizedError {
    case unavailable, invalidMessage, rateLimited, failed

    var errorDescription: String? {
        switch self {
        case .unavailable: return "Feedback is unavailable in this build."
        case .invalidMessage: return "Enter feedback between 1 and 4000 UTF-8 bytes."
        case .rateLimited: return "Too many submissions. Please try again later."
        case .failed: return "Feedback could not be sent. Please try again."
        }
    }
}

@MainActor
final class FeedbackService {
    private let endpoint: URL?
    private let session: URLSession
    private let redirectDelegate = FeedbackRedirectDelegate()

    init(endpoint: URL?, session: URLSession? = nil) {
        self.endpoint = endpoint.flatMap { Self.validEndpoint($0.absoluteString) }
        if let session {
            self.session = session
        } else {
            let configuration = URLSessionConfiguration.ephemeral
            configuration.httpShouldSetCookies = false
            configuration.httpCookieStorage = nil
            configuration.urlCache = nil
            configuration.urlCredentialStorage = nil
            self.session = URLSession(configuration: configuration)
        }
    }

    var isAvailable: Bool { endpoint != nil }

    static func validMessage(_ message: String) -> Bool {
        !message.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && message.utf8.count <= 4000
    }

    func submit(message: String) async throws {
        guard let endpoint else { throw FeedbackSubmissionError.unavailable }
        guard Self.validMessage(message) else { throw FeedbackSubmissionError.invalidMessage }
        struct Payload: Encodable { let message: String }
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.timeoutInterval = 15
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(Payload(message: message))
        let response: URLResponse
        do {
            (_, response) = try await session.data(for: request, delegate: redirectDelegate)
        } catch {
            throw FeedbackSubmissionError.failed
        }
        guard let http = response as? HTTPURLResponse else { throw FeedbackSubmissionError.failed }
        switch http.statusCode {
        case 202: return
        case 400: throw FeedbackSubmissionError.invalidMessage
        case 429: throw FeedbackSubmissionError.rateLimited
        default: throw FeedbackSubmissionError.failed
        }
    }

    static func configuredEndpoint(
        environment: [String: String] = ProcessInfo.processInfo.environment,
        analyticsEndpoint: String = MenubarBuildVersion.analyticsEndpoint
    ) -> URL? {
        if let override = environment["OCT_FEEDBACK_ENDPOINT"] {
            return validEndpoint(override)
        }
        guard let analytics = validEndpoint(analyticsEndpoint),
              var components = URLComponents(url: analytics, resolvingAgainstBaseURL: false),
              components.path.hasSuffix("/v1/events") else { return nil }
        components.path = String(components.path.dropLast("/v1/events".count)) + "/v1/feedback"
        return components.url
    }

    private static func validEndpoint(_ value: String) -> URL? {
        guard !value.isEmpty, !value.contains(where: { $0.isWhitespace }),
              let components = URLComponents(string: value),
              let host = components.host, !host.isEmpty,
              components.user == nil, components.password == nil,
              components.query == nil, components.fragment == nil,
              let url = components.url else { return nil }
        let scheme = components.scheme?.lowercased()
        let loopback = host == "127.0.0.1" || host == "::1" || host == "[::1]"
        guard scheme == "https" || (scheme == "http" && loopback) else { return nil }
        return url
    }
}
