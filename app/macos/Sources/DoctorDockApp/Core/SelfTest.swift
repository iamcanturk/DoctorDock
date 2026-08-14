import Foundation

/// A headless check of the Swift↔Go bridge.
///
///     DoctorDock --selftest
///
/// The interesting failures in this app are not in the views — they are in
/// decoding: a renamed field, a timestamp format the decoder does not accept,
/// a field that turned out to be optional on someone else's daemon. Those all
/// surface here, in a form that can run in a script, where a screenshot cannot
/// tell you anything.
enum SelfTest {

    static func run() async -> Int32 {
        var failures = 0

        func check(_ name: String, _ condition: Bool, _ detail: String = "") {
            if condition {
                print("  ✓ \(name)")
            } else {
                print("  ✗ \(name)\(detail.isEmpty ? "" : " — \(detail)")")
                failures += 1
            }
        }

        print("\nDoctorDock app self-test\n")

        // 1. Locating the engine.
        guard let binary = DoctorDockCLI.resolveBinary() else {
            print("  ✗ engine not found — neither bundled nor on PATH")
            return 1
        }
        print("  ✓ engine: \(binary.path)")

        // 2. Version, which also proves the process plumbing works.
        do {
            let version = try await DoctorDockCLI.version()
            check("version: \(version)", !version.isEmpty)
        } catch {
            check("version", false, describe(error))
            return 1
        }

        // 3. A full report. This is the decode that matters.
        let report: Report
        do {
            report = try await DoctorDockCLI.scan()
        } catch let failure as DoctorDockCLI.Failure where failure.isDockerDown {
            print("  – Docker is not running; skipping the report checks")
            print("\n\(failures == 0 ? "PASS" : "FAIL")\n")
            return failures == 0 ? 0 : 1
        } catch {
            check("scan", false, describe(error))
            return 1
        }

        check("schema \(report.schemaVersion) is supported",
              report.schemaVersion.hasPrefix(DoctorDockCLI.supportedSchemaMajor + "."))
        check("score \(report.score) is in range", (0...100).contains(report.score))
        check("docker: \(report.docker.display)", !report.docker.serverVersion.isEmpty)

        // The resource lists must be present — the app's tables are empty
        // without them, and `report` is the command that includes them.
        check("containers decoded (\(report.containers?.count ?? 0))", report.containers != nil)
        check("images decoded (\(report.images?.count ?? 0))", report.images != nil)
        check("volumes decoded (\(report.volumes?.count ?? 0))", report.volumes != nil)
        check("networks decoded (\(report.networks?.count ?? 0))", report.networks != nil)

        // Counts from the summary and the lists have to agree, or one of the
        // two decoded wrongly.
        check("summary matches the container list",
              report.summary.containers.total == (report.containers?.count ?? -1),
              "summary says \(report.summary.containers.total), list has \(report.containers?.count ?? -1)")
        check("summary matches the image list",
              report.summary.images.total == (report.images?.count ?? -1))

        // Every finding must carry what the UI renders.
        let incomplete = report.findings.filter {
            $0.ruleID.isEmpty || $0.rule.isEmpty || $0.title.isEmpty || $0.recommendation.isEmpty
        }
        check("all \(report.findings.count) findings are complete", incomplete.isEmpty,
              incomplete.first.map { "e.g. \($0.ruleID)" } ?? "")

        // Grouping must not lose or duplicate anything.
        let grouped = report.groupedFindings
        let regrouped = grouped.reduce(0) { $0 + $1.count }
        check("grouping preserves every finding (\(grouped.count) groups)",
              regrouped == report.findings.count,
              "\(regrouped) vs \(report.findings.count)")

        // Dates must have decoded, not silently become 1970.
        if let container = report.containers?.first {
            check("timestamps decoded", container.created.timeIntervalSince1970 > 946_684_800,
                  "\(container.name) created \(container.created)")
        }

        // The privacy guarantee, checked on real data.
        let leaked = report.containers?.flatMap(\.envKeys).filter { $0.contains("=") } ?? []
        check("no environment values in the report", leaked.isEmpty,
              leaked.first ?? "")

        // 4. A cleanup preview must never claim to have been applied.
        do {
            let plan = try await DoctorDockCLI.cleanupPreview(.safeDefaults, keepSince: nil)
            check("cleanup preview is a dry run (\(plan.items.count) items)", !plan.applied)
            check("preview reports no removals",
                  plan.summary.removed == 0 && !plan.items.contains { $0.removed })
        } catch {
            check("cleanup preview", false, describe(error))
        }

        // 5. The structured explanation the finding detail pane renders.
        do {
            let e = try await DoctorDockCLI.explanation("DD005")
            check("explanation DD005 decodes", e.id == "DD005" && e.severity == .critical)
            check("explanation has fixes with code (\(e.explanation.fixes.count))",
                  !e.explanation.fixes.isEmpty && e.explanation.fixes.allSatisfy { !$0.code.isEmpty })
            check("explanation has references",
                  !(e.explanation.references ?? []).isEmpty)
        } catch {
            check("explanation", false, describe(error))
        }

        // 6. The store must scan without anyone telling it to.
        //
        // This check exists because the first version had a start() method that
        // nothing called: every check above passed, the bridge was fine, and the
        // app still showed a spinner forever. Testing the bridge is not the same
        // as testing that anything uses it.
        let store = await ScanStore()
        let deadline = Date().addingTimeInterval(60)
        var reachedLoaded = false
        while Date() < deadline {
            let done = await MainActor.run { store.report != nil }
            if done { reachedLoaded = true; break }
            let stuckOnFailure = await MainActor.run {
                if case .failed = store.state { return true }
                return false
            }
            if stuckOnFailure { break }
            try? await Task.sleep(nanoseconds: 200_000_000)
        }
        check("the store scans on its own, with no start() to forget", reachedLoaded)

        print("\n\(failures == 0 ? "PASS" : "FAIL — \(failures) check(s) failed")\n")
        return failures == 0 ? 0 : 1
    }

    private static func describe(_ error: Error) -> String {
        (error as? DoctorDockCLI.Failure)?.errorDescription ?? error.localizedDescription
    }
}
