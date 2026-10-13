import XCTest

@testable import OctMenubarApp

/// The update prompt must only appear for a strictly newer stable release:
/// equal versions, downgrades, prerelease tags sharing a numeric core, and
/// unparseable local "dev" builds all have to stay silent.
final class ReleaseUpdateTests: XCTestCase {
    func testParseTagAcceptsVAndPrerelease() {
        XCTAssertEqual(ReleaseUpdateLogic.parseTag("v0.1.7"), [0, 1, 7])
        XCTAssertEqual(ReleaseUpdateLogic.parseTag("0.1.7"), [0, 1, 7])
        XCTAssertEqual(ReleaseUpdateLogic.parseTag("v0.1.7-beta.1"), [0, 1, 7])
        XCTAssertEqual(ReleaseUpdateLogic.parseTag(" v0.10.2 "), [0, 10, 2])
    }

    func testParseTagRejectsGarbage() {
        XCTAssertNil(ReleaseUpdateLogic.parseTag("dev"))
        XCTAssertNil(ReleaseUpdateLogic.parseTag(""))
        XCTAssertNil(ReleaseUpdateLogic.parseTag("v0.x.3"))
        XCTAssertNil(ReleaseUpdateLogic.parseTag("release-candidate"))
    }

    func testIsNewerComparesComponentWise() {
        XCTAssertTrue(ReleaseUpdateLogic.isNewer(current: [0, 1, 6], remote: [0, 1, 7]))
        XCTAssertTrue(ReleaseUpdateLogic.isNewer(current: [0, 1, 9], remote: [0, 2, 0]))
        XCTAssertFalse(ReleaseUpdateLogic.isNewer(current: [0, 1, 6], remote: [0, 1, 6]))
        XCTAssertFalse(ReleaseUpdateLogic.isNewer(current: [0, 1, 7], remote: [0, 1, 6]))
        XCTAssertFalse(ReleaseUpdateLogic.isNewer(current: [1, 0, 0], remote: [0, 9, 9]))
    }

    func testIsNewerPadsMissingComponentsWithZero() {
        XCTAssertFalse(ReleaseUpdateLogic.isNewer(current: [0, 1], remote: [0, 1, 0]))
        XCTAssertTrue(ReleaseUpdateLogic.isNewer(current: [0, 1], remote: [0, 1, 1]))
    }

    func testUpdateIfNewerPromptsOnlyForNewerRelease() {
        let update = ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.6",
            remoteTag: "v0.1.7",
            htmlURL: "https://github.com/suho-han/one-click-ai-tools/releases/tag/v0.1.7"
        )
        XCTAssertEqual(update?.tag, "v0.1.7")

        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.7", remoteTag: "v0.1.7", htmlURL: "https://example.com"
        ))
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.2.0", remoteTag: "v0.1.7", htmlURL: "https://example.com"
        ))
        // A prerelease sharing the numeric core is not an update prompt.
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.7", remoteTag: "v0.1.7-beta.1", htmlURL: "https://example.com"
        ))
        // Local "dev" builds never prompt.
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "dev", remoteTag: "v0.1.7", htmlURL: "https://example.com"
        ))
    }

    func testUpdateIfNewerRejectsUnusableDownloadURL() {
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.6", remoteTag: "v0.1.7", htmlURL: "not a url"
        ))
        XCTAssertNil(ReleaseUpdateLogic.updateIfNewer(
            current: "0.1.6", remoteTag: "v0.1.7", htmlURL: "ftp://example.com/v0.1.7"
        ))
    }

    @MainActor
    func testAutomaticUpdateCheckToggleHidesReleaseAndPersists() async {
        let suite = "oct-release-monitor-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [ReleaseStubProtocol.self]
        let session = URLSession(configuration: configuration)
        defer { session.invalidateAndCancel() }

        let monitor = ReleaseUpdateMonitor(currentVersion: "0.1.6", session: session, defaults: defaults)
        await monitor.checkNow()
        XCTAssertEqual(monitor.availableUpdate?.tag, "v0.1.7")

        monitor.setAutomaticallyChecksForUpdates(false)
        XCTAssertFalse(monitor.automaticallyChecksForUpdates)
        XCTAssertNil(monitor.availableUpdate)
        await monitor.checkNow()
        XCTAssertNil(monitor.availableUpdate)

        let reopened = ReleaseUpdateMonitor(currentVersion: "0.1.6", session: session, defaults: defaults)
        XCTAssertFalse(reopened.automaticallyChecksForUpdates)
        reopened.setAutomaticallyChecksForUpdates(true)
        await reopened.checkNow()
        XCTAssertEqual(reopened.availableUpdate?.tag, "v0.1.7")
    }
}

@MainActor
final class ReleaseUpdateInstallerTests: XCTestCase {
    func testSelfUpdateFailureReturnsCLIDiagnosticWithoutRetrying() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("oct-self-update-\(UUID())")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let executable = directory.appendingPathComponent("oct")
        let attempts = directory.appendingPathComponent("attempts")
        let script = """
        #!/bin/sh
        printf 'attempt\\n' >> '\(attempts.path)'
        if [ "$1" != "update" ] || [ "$2" != "--yes" ] || [ "$#" -ne 2 ]; then
            echo "incorrect update invocation" >&2
            exit 9
        fi
        echo "archive checksum mismatch" >&2
        exit 1
        """
        try script.write(to: executable, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: executable.path)
        let service = OctCLIService(executableURL: executable)

        do {
            _ = try await service.runSelfUpdate()
            XCTFail("a failed oct update must be shown as a failure")
        } catch let error as OctCLIServiceError {
            guard case .nonZeroExit(let status, let stderr) = error else {
                return XCTFail("expected CLI failure, got \(error)")
            }
            XCTAssertEqual(status, 1)
            XCTAssertTrue(stderr.contains("archive checksum mismatch"))
        }
        XCTAssertEqual(try String(contentsOf: attempts, encoding: .utf8), "attempt\n")
    }

    // MARK: performUpdate flow (dialog swapped out — a modal would block tests)

    private func makeUpdateScript(_ body: String) throws -> URL {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("oct-installer-\(UUID())")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        let executable = directory.appendingPathComponent("oct")
        try body.write(to: executable, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: executable.path)
        return executable
    }

    func testPerformUpdatePresentsFailureAndClearsIsUpdating() async throws {
        let executable = try makeUpdateScript("""
        #!/bin/sh
        echo "archive checksum mismatch" >&2
        exit 1
        """)
        defer { try? FileManager.default.removeItem(at: executable.deletingLastPathComponent()) }
        let installer = ReleaseUpdateInstaller(service: OctCLIService(executableURL: executable))
        var titles: [String] = []
        installer.presentResult = { titles.append($0.messageText) }

        await installer.performUpdate()

        XCTAssertFalse(installer.isUpdating, "a finished update must release the busy flag")
        XCTAssertEqual(titles, ["oct update failed"])
    }

    func testPerformUpdatePresentsSuccessWithCLIOutput() async throws {
        let executable = try makeUpdateScript("""
        #!/bin/sh
        echo "oct updated to v0.1.8"
        exit 0
        """)
        defer { try? FileManager.default.removeItem(at: executable.deletingLastPathComponent()) }
        let installer = ReleaseUpdateInstaller(service: OctCLIService(executableURL: executable))
        var titles: [String] = []
        installer.presentResult = { titles.append($0.messageText) }

        await installer.performUpdate()

        XCTAssertFalse(installer.isUpdating)
        XCTAssertEqual(titles, ["oct update finished"])
    }
}

private final class ReleaseStubProtocol: URLProtocol {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let response = HTTPURLResponse(
            url: request.url!,
            statusCode: 200,
            httpVersion: nil,
            headerFields: ["Content-Type": "application/json"]
        )!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(
            self,
            didLoad: Data(#"{"tag_name":"v0.1.7","html_url":"https://github.com/suho-han/one-click-ai-tools/releases/tag/v0.1.7"}"#.utf8)
        )
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}
