import Foundation

/// Runs the doctordock binary and decodes its JSON.
///
/// The app never talks to Docker itself. Every number it shows comes from the
/// same core the CLI uses, which is what stops the two ever disagreeing, and
/// means a crash in Docker handling cannot take down the menubar.
enum DoctorDockCLI {

    enum Failure: LocalizedError {
        case binaryNotFound
        case dockerUnreachable(String)
        case commandFailed(exitCode: Int32, stderr: String)
        case incompatibleSchema(found: String, supported: String)
        case decoding(String)

        var errorDescription: String? {
            switch self {
            case .binaryNotFound:
                return "The doctordock binary is missing from the app bundle and is not on PATH."
            case .dockerUnreachable(let detail):
                return detail.isEmpty ? "Docker is not running." : detail
            case .commandFailed(let code, let stderr):
                return stderr.isEmpty ? "doctordock exited with code \(code)." : stderr
            case .incompatibleSchema(let found, let supported):
                return "This app understands report schema \(supported).x but doctordock produced \(found). Update the app."
            case .decoding(let detail):
                return "Could not read the report: \(detail)"
            }
        }

        /// True when the failure is "Docker is not running", which is a normal
        /// state to be in rather than something to show as a crash.
        var isDockerDown: Bool {
            if case .dockerUnreachable = self { return true }
            return false
        }
    }

    /// The report schema major this app was written against. A minor bump adds
    /// fields, which decoding tolerates; a major bump does not.
    static let supportedSchemaMajor = "1"

    // MARK: - Locating the binary

    /// Finds the doctordock binary.
    ///
    /// A copy is bundled so the app works with nothing else installed. A
    /// binary on PATH wins when present, so `brew upgrade doctordock` improves
    /// the app too — the schema check below catches the case where that
    /// upgrade went too far ahead.
    static func resolveBinary() -> URL? {
        for candidate in pathCandidates() where FileManager.default.isExecutableFile(atPath: candidate.path) {
            return candidate
        }
        if let bundled = Bundle.main.url(forResource: "doctordock", withExtension: nil),
           FileManager.default.isExecutableFile(atPath: bundled.path) {
            return bundled
        }
        return nil
    }

    private static func pathCandidates() -> [URL] {
        // The app is launched by Finder, which gives it a minimal PATH that
        // does not include Homebrew. Looking in the usual places directly is
        // more reliable than trusting the inherited environment.
        var dirs = ["/opt/homebrew/bin", "/usr/local/bin"]
        if let home = ProcessInfo.processInfo.environment["HOME"] {
            dirs.append("\(home)/.local/bin")
            dirs.append("\(home)/go/bin")
        }
        if let path = ProcessInfo.processInfo.environment["PATH"] {
            dirs.append(contentsOf: path.split(separator: ":").map(String.init))
        }
        return dirs.map { URL(fileURLWithPath: $0).appendingPathComponent("doctordock") }
    }

    // MARK: - Running

    /// Runs doctordock with the given arguments and returns stdout.
    ///
    /// Non-zero exit codes are not automatically failures: `scan --fail-on`
    /// uses 1, 2 and 3 to report findings and still writes a complete report.
    /// Only 10 — the tool itself failing — is an error.
    static func run(_ arguments: [String], timeout: TimeInterval = 90) async throws -> Data {
        guard let binary = resolveBinary() else { throw Failure.binaryNotFound }

        return try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                let process = Process()
                process.executableURL = binary
                process.arguments = arguments + ["--no-color"]

                // A GUI process inherits almost no environment. DOCKER_HOST and
                // friends are how people point at Colima, OrbStack or a remote
                // daemon, so anything already set is passed through, and PATH is
                // widened so the binary can find a docker context if it needs one.
                var env = ProcessInfo.processInfo.environment
                env["PATH"] = "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:" + (env["PATH"] ?? "")
                process.environment = env

                let stdout = Pipe()
                let stderr = Pipe()
                process.standardOutput = stdout
                process.standardError = stderr

                Log.info("run: doctordock " + arguments.joined(separator: " "))
                do {
                    try process.run()
                } catch {
                    Log.error("could not start doctordock: " + error.localizedDescription)
                    continuation.resume(throwing: Failure.commandFailed(
                        exitCode: -1, stderr: error.localizedDescription))
                    return
                }

                // Read both pipes before waiting. A report of any size fills the
                // 64 KB pipe buffer, and a process blocked on a full pipe never
                // exits — this is the classic deadlock.
                let outData = stdout.fileHandleForReading.readDataToEndOfFile()
                let errData = stderr.fileHandleForReading.readDataToEndOfFile()

                let deadline = Date().addingTimeInterval(timeout)
                while process.isRunning && Date() < deadline {
                    Thread.sleep(forTimeInterval: 0.05)
                }
                if process.isRunning {
                    process.terminate()
                    continuation.resume(throwing: Failure.commandFailed(
                        exitCode: -1, stderr: "doctordock did not finish within \(Int(timeout))s"))
                    return
                }

                let errText = String(data: errData, encoding: .utf8) ?? ""

                // Exit 10 is DoctorDock itself failing — bad flags, unreadable
                // config, or no daemon. Everything else still produced output.
                if process.terminationStatus == 10 {
                    if errText.localizedCaseInsensitiveContains("docker daemon") ||
                        errText.localizedCaseInsensitiveContains("cannot reach") {
                        continuation.resume(throwing: Failure.dockerUnreachable(firstLine(of: errText)))
                    } else {
                        continuation.resume(throwing: Failure.commandFailed(
                            exitCode: 10, stderr: firstLine(of: errText)))
                    }
                    return
                }

                if outData.isEmpty {
                    continuation.resume(throwing: Failure.commandFailed(
                        exitCode: process.terminationStatus, stderr: firstLine(of: errText)))
                    return
                }

                continuation.resume(returning: outData)
            }
        }
    }

    private static func firstLine(of text: String) -> String {
        text.split(separator: "\n")
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .first(where: { !$0.isEmpty && !$0.hasPrefix("Error:") })
            ?? text.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    // MARK: - Decoding

    static let decoder: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        // The core emits RFC 3339 with fractional seconds, which .iso8601 does
        // not accept. Both forms are tried because `created` fields from the
        // daemon often have none.
        decoder.dateDecodingStrategy = .custom { decoder in
            let text = try decoder.singleValueContainer().decode(String.self)
            for formatter in [fractionalFormatter, plainFormatter] {
                if let date = formatter.date(from: text) { return date }
            }
            throw DecodingError.dataCorrupted(.init(
                codingPath: decoder.codingPath,
                debugDescription: "unrecognised timestamp: \(text)"))
        }
        return decoder
    }()

    private static let fractionalFormatter: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f
    }()

    private static let plainFormatter: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        return f
    }()

    private static func decode<T: Decodable>(_ type: T.Type, from data: Data) throws -> T {
        do {
            return try decoder.decode(type, from: data)
        } catch {
            throw Failure.decoding(String(describing: error))
        }
    }

    /// Refuses a document whose major schema version this app does not
    /// understand, rather than silently showing wrong numbers.
    private static func checkSchema(_ version: String) throws {
        let major = version.split(separator: ".").first.map(String.init) ?? version
        guard major == supportedSchemaMajor else {
            throw Failure.incompatibleSchema(found: version, supported: supportedSchemaMajor)
        }
    }

    // MARK: - Commands

    /// A full report, including every resource list.
    static func scan() async throws -> Report {
        let data = try await run(["report", "--format", "json"])
        let report = try decode(Report.self, from: data)
        try checkSchema(report.schemaVersion)
        return report
    }

    /// What a cleanup would remove. Never deletes anything.
    static func cleanupPreview(_ targets: CleanupTargets, keepSince: String?) async throws -> CleanupPlan {
        var args = ["cleanup", "--format", "json"] + targets.flags
        if let keepSince, !keepSince.isEmpty {
            args += ["--keep-since", keepSince]
        }
        let data = try await run(args)
        let plan = try decode(CleanupPlan.self, from: data)
        try checkSchema(plan.schemaVersion)
        return plan
    }

    /// Performs the cleanup.
    ///
    /// `--yes` is passed because the user has already confirmed in the app's
    /// own dialog; the CLI's terminal prompt cannot be answered from here. The
    /// targets are still explicit, so volumes are only ever included when the
    /// user asked for them.
    static func cleanupApply(_ targets: CleanupTargets, keepSince: String?) async throws -> CleanupPlan {
        var args = ["cleanup", "--format", "json", "--apply", "--yes"] + targets.flags
        if let keepSince, !keepSince.isEmpty {
            args += ["--keep-since", keepSince]
        }
        let data = try await run(args, timeout: 300)
        let plan = try decode(CleanupPlan.self, from: data)
        try checkSchema(plan.schemaVersion)
        return plan
    }

    /// The long-form explanation of a rule, as plain text.
    static func explain(_ ruleID: String) async throws -> String {
        let data = try await run(["explain", ruleID])
        return String(data: data, encoding: .utf8) ?? ""
    }

    /// The structured explanation of a rule, for native rendering.
    static func explanation(_ ruleID: String) async throws -> RuleExplanation {
        let data = try await run(["explain", ruleID, "--format", "json"])
        return try decode(RuleExplanation.self, from: data)
    }

    /// The version of the binary actually being used.
    static func version() async throws -> String {
        let data = try await run(["version", "--format", "json"])
        struct VersionInfo: Decodable { let version: String }
        return try decode(VersionInfo.self, from: data).version
    }
}
