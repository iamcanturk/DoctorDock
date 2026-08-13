// swift-tools-version: 5.9
import PackageDescription

// DoctorDock's macOS app is a Swift package rather than an Xcode project.
//
// That is a deliberate choice: `swift build` works with only the Command Line
// Tools, so nobody has to install Xcode — a 15 GB download — to build or
// contribute to it. Package.swift also opens directly in Xcode for anyone who
// does have it, and there is no generated .xcodeproj to churn in git.
//
// The tools version is 5.9 rather than 6.x so that the package builds on the
// Swift that ships with current Command Line Tools, and so the Swift 5 language
// mode applies: the app runs a subprocess and updates the UI from its result,
// which Swift 6's strict concurrency checking makes noisy for no safety gain
// here — every UI type is already @MainActor.
//
// SPM produces a bare executable, not a .app bundle. scripts/build-app.sh
// assembles the bundle around it.
let package = Package(
    name: "DoctorDockApp",
    platforms: [.macOS(.v14)],
    targets: [
        .executableTarget(
            name: "DoctorDockApp",
            path: "Sources/DoctorDockApp"
        ),
    ]
)
