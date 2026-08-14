import SwiftUI
import AppKit

/// Dumps the app's views to PNG for design review.
///
///     DoctorDock --render /tmp/dd-previews
///
/// This is not shipped UI; it exists so the design can be looked at as images.
///
/// ImageRenderer draws into a plain bitmap with no window and no material, so
/// each app view is given a solid background approximating the popover/window
/// material and a fixed colour scheme. Otherwise `.primary` text renders black
/// on a transparent (→ black) canvas, which tells you nothing about the design.
@MainActor func renderPreviews(to dir: String) -> Int32 {
    let fm = FileManager.default
    try? fm.createDirectory(atPath: dir, withIntermediateDirectories: true)

    let report = SampleData.report
    let store = ScanStore.preview(report)

    let explanation: RuleExplanation? = SampleData.explanation

    // Approximations of the AppKit materials, per scheme.
    func popoverBG(_ scheme: ColorScheme) -> Color {
        scheme == .dark ? Color(red: 0.13, green: 0.13, blue: 0.14)
                        : Color(red: 0.96, green: 0.96, blue: 0.97)
    }
    func windowBG(_ scheme: ColorScheme) -> Color {
        scheme == .dark ? Color(red: 0.11, green: 0.11, blue: 0.12)
                        : Color(red: 0.93, green: 0.93, blue: 0.94)
    }

    var jobs: [(String, AnyView, CGFloat)] = [
        ("share-card", AnyView(ShareCard(score: report.score, summary: report.summary,
            showPlatform: true, platform: report.docker.operatingSystem ?? "")), 1),
    ]
    var viewSizes: [String: CGSize] = [:]

    // A standalone finding detail with the explanation injected, so the fixes,
    // code blocks and references design can be reviewed.
    if let socket = report.groupedFindings.first(where: { $0.ruleID == "DD005" }) {
        viewSizes["finding-detail"] = CGSize(width: 720, height: 900)
        jobs.append(("finding-detail", AnyView(
            FindingDetailView(group: socket, preloaded: explanation)
                .background(Color(red: 0.11, green: 0.11, blue: 0.12))
                .environment(\.colorScheme, .dark)), 2))
    }

    // Resource tables (dark only, to keep the set small).
    for (name, view) in [
        ("containers", AnyView(ContainersView(report: report))),
        ("images", AnyView(ImagesView(report: report))),
        ("volumes", AnyView(VolumesView(report: report))),
        ("networks", AnyView(NetworksView(report: report))),
    ] {
        viewSizes[name] = CGSize(width: 820, height: 460)
        jobs.append((name, AnyView(
            view.background(Color(red: 0.12, green: 0.12, blue: 0.13))
                .environment(\.colorScheme, .dark)), 2))
    }

    // The share sheet, hosted (it contains AppKit controls).
    viewSizes["share-sheet"] = CGSize(width: 560, height: 720)
    jobs.append(("share-sheet", AnyView(
        ShareSheet(report: report)
            .environment(\.colorScheme, .dark)), 2))

    // App views in both schemes, so contrast problems in either are visible.
    for scheme in [ColorScheme.dark, .light] {
        let tag = scheme == .dark ? "dark" : "light"
        viewSizes["popover-\(tag)"] = CGSize(width: 360, height: 560)
        jobs.append(("popover-\(tag)", AnyView(
            PopoverView().environmentObject(store)
                .background(popoverBG(scheme))
                .environment(\.colorScheme, scheme)), 2))
        viewSizes["overview-\(tag)"] = CGSize(width: 760, height: 680)
        jobs.append(("overview-\(tag)", AnyView(
            OverviewView(report: report)
                .background(windowBG(scheme))
                .environment(\.colorScheme, scheme)), 2))
        viewSizes["findings-\(tag)"] = CGSize(width: 900, height: 600)
        jobs.append(("findings-\(tag)", AnyView(
            FindingsView(report: report)
                .background(windowBG(scheme))
                .environment(\.colorScheme, scheme)), 2))
        viewSizes["cleanup-\(tag)"] = CGSize(width: 820, height: 600)
        jobs.append(("cleanup-\(tag)", AnyView(
            CleanupView().environmentObject(store)
                .background(windowBG(scheme))
                .environment(\.colorScheme, scheme)), 2))
    }

    var failures = 0
    for (name, view, scale) in jobs {
        // The share card is flat; ImageRenderer handles it and honours scale.
        // Everything else contains scroll/list/split views, which only a real
        // hosted AppKit hierarchy lays out.
        let data: Data?
        if name == "share-card" {
            data = CardRenderer.png(view, scale: scale)
        } else {
            let size = viewSizes[name] ?? CGSize(width: 800, height: 600)
            // Findings and cleanup fetch content asynchronously; let it settle.
            let settle: TimeInterval = name.hasPrefix("findings") ? 2.0 : 0
            data = CardRenderer.hosted(view, size: size, settle: settle)
        }
        if let data {
            let path = "\(dir)/\(name).png"
            try? data.write(to: URL(fileURLWithPath: path))
            print("  wrote \(path) (\(data.count / 1024) KB)")
        } else {
            print("  FAILED to render \(name)")
            failures += 1
        }
    }
    return failures == 0 ? 0 : 1
}
