import Combine
import Foundation

/// A newer GitHub release of oct, ready to surface in the menubar.
struct ReleaseUpdate: Equatable {
    let tag: String
    let htmlURL: URL
}

/// Pure release-comparison logic, kept free of networking so tests can
/// exercise it directly: parse a "vMAJOR.MINOR.PATCH[-prerelease]" tag into
/// its numeric core and decide whether it is strictly newer than the
/// running build's version.
enum ReleaseUpdateLogic {
    /// Numeric core of a release tag; nil for anything unparseable, which
    /// includes local "dev" builds so they never trigger an update prompt.
    static func parseTag(_ tag: String) -> [Int]? {
        var core = Substring(tag.trimmingCharacters(in: .whitespaces))
        if core.first == "v" || core.first == "V" {
            core = core.dropFirst()
        }
        // Drop any prerelease suffix ("0.1.7-beta.1"): `releases/latest`
        // never returns prereleases, and a local prerelease sharing the
        // numeric core should not prompt.
        if let dash = core.firstIndex(of: "-") {
            core = core[..<dash]
        }
        var parts: [Int] = []
        for piece in core.split(separator: ".") {
            guard let number = Int(piece) else { return nil }
            parts.append(number)
        }
        return parts.isEmpty ? nil : parts
    }

    /// Element-wise comparison; missing components count as zero, so 0.1 and
    /// 0.1.0 compare equal.
    static func isNewer(current: [Int], remote: [Int]) -> Bool {
        let length = max(current.count, remote.count)
        for index in 0..<length {
            let lhs = index < current.count ? current[index] : 0
            let rhs = index < remote.count ? remote[index] : 0
            if lhs != rhs {
                return lhs < rhs
            }
        }
        return false
    }

    /// nil unless the remote tag is strictly newer than `current` and the
    /// release page is an http(s) URL.
    static func updateIfNewer(current: String, remoteTag: String, htmlURL: String) -> ReleaseUpdate? {
        guard let lhs = parseTag(current),
              let rhs = parseTag(remoteTag),
              isNewer(current: lhs, remote: rhs),
              let url = URL(string: htmlURL.trimmingCharacters(in: .whitespaces)),
              let scheme = url.scheme?.lowercased(),
              scheme == "https" || scheme == "http"
        else {
            return nil
        }
        return ReleaseUpdate(tag: remoteTag.trimmingCharacters(in: .whitespaces), htmlURL: url)
    }
}

/// Polls the GitHub Releases API every five minutes and publishes the newer
/// release, if any. Failures keep the last known state: one offline interval
/// or a rate-limited response must not flicker the update button away.
@MainActor
final class ReleaseUpdateMonitor: ObservableObject {
    static let repo = "suho-han/one-click-ai-tools"
    static let pollInterval: TimeInterval = 5 * 60

    @Published private(set) var availableUpdate: ReleaseUpdate?

    private var timer: Timer?
    private let currentVersion: String
    private let session: URLSession

    init(currentVersion: String = MenubarBuildVersion.version, session: URLSession = .shared) {
        self.currentVersion = currentVersion
        self.session = session
    }

    func start() {
        Task { await checkNow() }
        timer = Timer.scheduledTimer(withTimeInterval: Self.pollInterval, repeats: true) { [weak self] _ in
            Task { await self?.checkNow() }
        }
    }

    func checkNow() async {
        guard let url = URL(string: "https://api.github.com/repos/\(Self.repo)/releases/latest") else { return }
        var request = URLRequest(url: url)
        request.timeoutInterval = 15
        request.setValue("application/vnd.github+json", forHTTPHeaderField: "Accept")
        request.setValue("one-click-ai-tools-menubar", forHTTPHeaderField: "User-Agent")
        do {
            let (data, _) = try await session.data(for: request)
            struct Payload: Decodable {
                let tagName: String
                let htmlUrl: String
                enum CodingKeys: String, CodingKey {
                    case tagName = "tag_name"
                    case htmlUrl = "html_url"
                }
            }
            let payload = try JSONDecoder().decode(Payload.self, from: data)
            availableUpdate = ReleaseUpdateLogic.updateIfNewer(
                current: currentVersion,
                remoteTag: payload.tagName,
                htmlURL: payload.htmlUrl
            )
        } catch {
            // Network errors, rate limits, and malformed responses all land
            // here; the menubar keeps showing what it knew before.
        }
    }
}
