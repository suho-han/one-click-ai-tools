import XCTest

@testable import OctMenubarApp

final class RestartHelperTests: XCTestCase {
    func testHelperRestartScriptStopsThenStartsDaemon() {
        let script = OctCLIService.helperRestartScript(octPath: "/usr/local/bin/oct")

        XCTAssertTrue(script.contains("'/usr/local/bin/oct' menubar stop"), "restart should stop first")
        XCTAssertTrue(script.contains("menubar --daemon"), "restart should start the detached helper after stopping")
        let stopRange = script.range(of: "menubar stop")!
        let startRange = script.range(of: "menubar --daemon")!
        XCTAssertLessThan(stopRange.lowerBound, startRange.lowerBound, "stop must run before start")
    }

    func testHelperRestartScriptQuotesPathsWithSpaces() {
        let script = OctCLIService.helperRestartScript(octPath: "/Users/some one/tools/oct")

        XCTAssertTrue(script.contains("'/Users/some one/tools/oct'"), "oct path must be shell-quoted")
    }

    func testFooterSplitsRowBetweenRestartAndQuit() throws {
        let testFile = URL(fileURLWithPath: #filePath)
        let packageRoot = testFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let footerPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/Views/FooterActionsView.swift")
        let footerSource = try String(contentsOf: footerPath, encoding: .utf8)

        XCTAssertTrue(footerSource.contains("Restart helper"), "footer should offer a restart helper action")
        XCTAssertTrue(footerSource.contains("Quit helper"), "footer should keep the quit helper action")
        let restartRange = footerSource.range(of: "Restart helper")!
        let quitRange = footerSource.range(of: "Quit helper")!
        XCTAssertLessThan(restartRange.lowerBound, quitRange.lowerBound, "restart should precede quit in the shared row")
        XCTAssertTrue(footerSource.contains("onRestartHelper"), "restart action should be caller-injected like refresh")
    }
}
