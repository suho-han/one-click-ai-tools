import XCTest
@testable import OctMenubarApp

final class FeedbackTests: XCTestCase {
    @MainActor
    private func store(status: String) -> (FeedbackStore, URLSession) {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [FeedbackStubProtocol.self]
        let session = URLSession(configuration: configuration)
        return (FeedbackStore(service: FeedbackService(endpoint: URL(string: "https://feedback.test/\(status)/\(UUID().uuidString)"), session: session)), session)
    }

    @MainActor
    func testSuccessClearsDraftAndDuplicateSubmitDoesNotSendTwice() async {
        let gate = "held-\(UUID().uuidString)"
        let (store, session) = store(status: gate)
        defer { session.invalidateAndCancel() }
        store.message = "  한글\nfeedback "
        let first = Task { await store.submit() }
        while !store.isSubmitting && store.status == nil { await Task.yield() }
        XCTAssertTrue(store.isSubmitting)
        await store.submit()
        FeedbackStubProtocol.release(gate)
        await first.value
        XCTAssertEqual(store.message, "")
        XCTAssertEqual(store.status?.message, "Feedback sent. Thank you!")
        XCTAssertFalse(store.isSubmitting)
        // A second request for this unique URL fails in the protocol fixture.
        store.message = "second"
        XCTAssertNil(store.status)
        await store.submit()
        XCTAssertEqual(store.message, "second")
        XCTAssertEqual(store.status?.kind, .error)
    }

    @MainActor
    func testFailureRetainsDraft() async {
        for status in ["400", "429", "503", "200", "302", "timeout"] {
            let (store, session) = store(status: status)
            store.message = "한글 draft"
            await store.submit()
            XCTAssertEqual(store.message, "한글 draft", status)
            XCTAssertEqual(store.status?.kind, .error, status)
            XCTAssertFalse(store.isSubmitting)
            session.invalidateAndCancel()
        }
    }

    @MainActor
    func testByteBoundaryAndUnavailableGuard() async {
        let (store, session) = store(status: "202")
        defer { session.invalidateAndCancel() }
        store.message = String(repeating: "한", count: 1333) + "a"
        XCTAssertEqual(store.messageByteCount, 4000)
        XCTAssertTrue(store.canSubmit)
        store.message += "a"
        XCTAssertFalse(store.canSubmit)
        await store.submit()
        XCTAssertNil(store.status)
        store.message = " \n\t"
        XCTAssertFalse(store.canSubmit)
        let unavailable = FeedbackStore(service: FeedbackService(endpoint: URL(string: "http://example.com/v1/feedback"), session: session))
        unavailable.message = "draft"
        await unavailable.submit()
        XCTAssertFalse(unavailable.canSubmit)
        XCTAssertEqual(unavailable.message, "draft")
        XCTAssertNil(unavailable.status)
    }

    @MainActor
    func testEndpointPrecedencePrivacyAndOptOut() async {
        let base = "https://azure.example:8443/v1/events"
        XCTAssertEqual(FeedbackService.configuredEndpoint(environment: [:], analyticsEndpoint: base)?.absoluteString, "https://azure.example:8443/v1/feedback")
        for invalid in ["", "http://example.com/v1/feedback", "https://u:p@example.com/v1/feedback", "https://example.com/v1/feedback?q=1", "https://example.com/v1/feedback#fragment", "not a url"] {
            XCTAssertNil(FeedbackService.configuredEndpoint(environment: ["OCT_FEEDBACK_ENDPOINT": invalid], analyticsEndpoint: base))
        }
        XCTAssertNil(FeedbackService.configuredEndpoint(environment: [:], analyticsEndpoint: "https://example.com/other"))
        XCTAssertNil(FeedbackService.configuredEndpoint(environment: [:], analyticsEndpoint: ""))
        for loopback in ["http://127.0.0.1:8843/v1/feedback", "http://[::1]:8843/v1/feedback"] {
            XCTAssertNotNil(FeedbackService.configuredEndpoint(environment: ["OCT_FEEDBACK_ENDPOINT": loopback], analyticsEndpoint: base))
        }
        let endpoint = FeedbackService.configuredEndpoint(environment: ["OCT_FEEDBACK_ENDPOINT": "https://feedback.test/202/\(UUID().uuidString)", "DO_NOT_TRACK": "1", "OCT_TELEMETRY_ENABLED": "false", "CI": "true"], analyticsEndpoint: base)
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [FeedbackStubProtocol.self]
        let session = URLSession(configuration: configuration)
        defer { session.invalidateAndCancel() }
        let store = FeedbackStore(service: FeedbackService(endpoint: endpoint, session: session))
        store.message = "explicit feedback"
        await store.submit()
        XCTAssertEqual(store.status?.kind, .success)
    }
}

private final class FeedbackStubProtocol: URLProtocol, @unchecked Sendable {
    private static let lock = NSLock()
    nonisolated(unsafe) private static var seen = Set<URL>()
    private static let condition = NSCondition()
    nonisolated(unsafe) private static var released = Set<String>()

    static func release(_ gate: String) {
        condition.lock()
        released.insert(gate)
        condition.broadcast()
        condition.unlock()
    }
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        let url = request.url!
        Self.lock.lock()
        let first = Self.seen.insert(url).inserted
        Self.lock.unlock()
        guard first else { client?.urlProtocol(self, didFailWithError: URLError(.badServerResponse)); return }
        let status = url.pathComponents[1]
        if status == "timeout" { client?.urlProtocol(self, didFailWithError: URLError(.timedOut)); return }
        if status.hasPrefix("held-") {
            Self.condition.lock()
            while !Self.released.contains(status) { Self.condition.wait() }
            Self.condition.unlock()
        }
        let response = HTTPURLResponse(url: url, statusCode: status.hasPrefix("held-") ? 202 : Int(status)!, httpVersion: nil, headerFields: ["Location": "https://other.test/"])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: Data("private server response".utf8))
        client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}
