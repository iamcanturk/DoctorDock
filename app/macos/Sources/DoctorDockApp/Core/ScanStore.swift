import Foundation
import SwiftUI
import UserNotifications

/// The app's single source of truth: the latest report, when it was taken, and
/// what went wrong if it could not be.
@MainActor
final class ScanStore: ObservableObject {

    enum State {
        case idle
        case scanning
        case loaded(Report)
        case failed(DoctorDockCLI.Failure)

        var report: Report? {
            if case .loaded(let report) = self { return report }
            return nil
        }
    }

    @Published private(set) var state: State = .idle
    @Published private(set) var lastUpdated: Date?
    @Published private(set) var binaryVersion: String?

    /// The previous score, kept so a drop can be noticed. Nil until the second
    /// successful scan.
    @Published private(set) var previousScore: Int?

    @AppStorage("refreshInterval") var refreshInterval: TimeInterval = 300
    @AppStorage("notifyOnScoreDrop") var notifyOnScoreDrop: Bool = true
    @AppStorage("notifyOnNewCritical") var notifyOnNewCritical: Bool = true

    private var timer: Timer?
    private var isRefreshing = false
    /// Rule IDs already notified about, so one unfixed problem does not
    /// produce a notification every five minutes.
    private var notifiedCritical: Set<String> = []

    var report: Report? { state.report }

    var score: Int? { report?.score }

    var isScanning: Bool {
        if case .scanning = state { return true }
        return false
    }

    // MARK: - Lifecycle

    convenience init() {
        self.init(skipAutoScan: false)
    }

    /// A store preloaded with a report, for previews and the renderer.
    static func preview(_ report: Report) -> ScanStore {
        let store = ScanStore(skipAutoScan: true)
        store.loadForPreview(report)
        return store
    }

    /// The designated initialiser. `skipAutoScan` is for previews and the
    /// headless renderer, which load a fixed report instead of scanning.
    init(skipAutoScan: Bool) {
        guard !skipAutoScan else { return }

        // The store scans as soon as it exists, rather than waiting for a view
        // to call a start method.
        //
        // The first version had a start() that nothing called: the app sat in
        // .idle forever, which rendered as a spinner, so it looked like a hung
        // scan rather than a scan that never began. Removing the hook removes
        // the failure mode — there is nothing left to forget.
        Task { @MainActor in
            Log.info("store created, starting the first scan")
            self.scheduleTimer()
            await self.refresh()
            self.binaryVersion = try? await DoctorDockCLI.version()
        }
    }

    /// Loads a fixed report for previews, bypassing the daemon.
    func loadForPreview(_ report: Report) {
        state = .loaded(report)
        lastUpdated = report.generatedAt
        binaryVersion = report.tool.version
    }

    func scheduleTimer() {
        timer?.invalidate()
        guard refreshInterval > 0 else { return }
        timer = Timer.scheduledTimer(withTimeInterval: refreshInterval, repeats: true) { [weak self] _ in
            Task { @MainActor in await self?.refresh() }
        }
    }

    // MARK: - Refreshing

    func refresh() async {
        // A scan takes about half a second, but a busy machine or a wedged
        // daemon can stretch that past the refresh interval. Overlapping scans
        // would stack up.
        guard !isRefreshing else { return }
        isRefreshing = true
        defer { isRefreshing = false }

        // A refresh that fails should not blank a report the user is reading,
        // so the previous one stays on screen until a new one replaces it.
        if state.report == nil {
            state = .scanning
        }

        let started = Date()
        do {
            let report = try await DoctorDockCLI.scan()
            let earlier = state.report?.score
            Log.info("scan finished in \(String(format: "%.2f", Date().timeIntervalSince(started)))s — score \(report.score), \(report.findings.count) findings")

            state = .loaded(report)
            lastUpdated = Date()

            if let earlier {
                previousScore = earlier
                await notifyIfWorse(previous: earlier, report: report)
            }
            pruneNotifiedCritical(against: report)
        } catch let failure as DoctorDockCLI.Failure {
            Log.error("scan failed: " + (failure.errorDescription ?? "unknown"))
            state = .failed(failure)
        } catch {
            Log.error("scan failed: " + error.localizedDescription)
            state = .failed(.commandFailed(exitCode: -1, stderr: error.localizedDescription))
        }
    }

    // MARK: - Notifications

    private func notifyIfWorse(previous: Int, report: Report) async {
        if notifyOnScoreDrop, report.score < previous - 5 {
            await Notifier.send(
                title: "Docker health dropped",
                body: "Score fell from \(previous) to \(report.score)."
            )
        }

        guard notifyOnNewCritical else { return }

        // Only genuinely new critical findings are worth interrupting for.
        let criticals = report.findings.filter { $0.severity == .critical }
        let fresh = criticals.filter { !notifiedCritical.contains($0.id) }
        guard let first = fresh.first else { return }

        notifiedCritical.formUnion(fresh.map(\.id))
        let body = fresh.count == 1
            ? "\(first.rule) — \(first.resourceName)"
            : "\(fresh.count) new critical findings, including \(first.rule)"
        await Notifier.send(title: "Critical Docker finding", body: body)
    }

    /// Forgets findings that have been fixed, so the same problem recurring
    /// later notifies again.
    private func pruneNotifiedCritical(against report: Report) {
        let current = Set(report.findings.filter { $0.severity == .critical }.map(\.id))
        notifiedCritical.formIntersection(current)
    }
}

/// Wraps UNUserNotificationCenter so that an app running without notification
/// permission — or unsigned, where the framework can refuse outright — degrades
/// to silence rather than crashing.
enum Notifier {
    private static var authorized = false
    private static var didRequest = false

    static func requestAuthorizationIfNeeded() async {
        guard !didRequest else { return }
        didRequest = true

        guard Bundle.main.bundleIdentifier != nil else { return }
        let center = UNUserNotificationCenter.current()
        authorized = (try? await center.requestAuthorization(options: [.alert, .sound])) ?? false
    }

    static func send(title: String, body: String) async {
        await requestAuthorizationIfNeeded()
        guard authorized else { return }

        let content = UNMutableNotificationContent()
        content.title = title
        content.body = body
        content.sound = .default

        let request = UNNotificationRequest(
            identifier: UUID().uuidString, content: content, trigger: nil)
        try? await UNUserNotificationCenter.current().add(request)
    }
}
