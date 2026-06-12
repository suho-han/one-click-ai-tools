// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "OctMenubar",
    platforms: [
        .macOS(.v14),
    ],
    products: [
        .executable(name: "OctMenubarApp", targets: ["OctMenubarApp"]),
    ],
    targets: [
        .executableTarget(
            name: "OctMenubarApp"
        ),
        .testTarget(
            name: "OctMenubarTests",
            dependencies: ["OctMenubarApp"]
        ),
    ]
)
