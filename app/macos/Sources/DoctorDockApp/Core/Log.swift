import Foundation
import os

/// Diagnostics for an app that has no console.
///
/// A menubar app launched from Finder has nowhere to print. When the first
/// version never ran its first scan, the symptom — a spinner that never
/// stopped — was indistinguishable from a scan that hung, and there was no way
/// to tell which without a debugger. This is that missing information.
///
/// It writes to both the unified system log and a plain file, because the
/// unified log turned out not to surface reliably for an ad-hoc signed bundle,
/// and a diagnostic you cannot read is not a diagnostic.
///
///     tail -f ~/Library/Logs/DoctorDock.log
enum Log {
    private static let osLog = Logger(subsystem: "dev.iamcanturk.doctordock", category: "app")

    /// Serializes writes; the log is touched from the scan task and the UI.
    private static let queue = DispatchQueue(label: "dev.iamcanturk.doctordock.log")

    private static let fileURL: URL? = {
        guard let logs = try? FileManager.default.url(
            for: .libraryDirectory, in: .userDomainMask, appropriateFor: nil, create: false
        ).appendingPathComponent("Logs") else { return nil }
        try? FileManager.default.createDirectory(at: logs, withIntermediateDirectories: true)
        return logs.appendingPathComponent("DoctorDock.log")
    }()

    /// Above this the file is truncated. A diagnostic log that fills a disk is
    /// a worse bug than the one it was added to find.
    private static let maxBytes = 512 * 1024

    static func info(_ message: String) {
        osLog.info("\(message, privacy: .public)")
        write("INFO", message)
    }

    static func error(_ message: String) {
        osLog.error("\(message, privacy: .public)")
        write("ERROR", message)
    }

    /// The path, for showing the user where to look.
    static var path: String { fileURL?.path ?? "unavailable" }

    private static let timestamp: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        return f
    }()

    private static func write(_ level: String, _ message: String) {
        guard let fileURL else { return }
        let line = "\(timestamp.string(from: Date())) \(level) \(message)\n"

        queue.async {
            guard let data = line.data(using: .utf8) else { return }

            if let handle = try? FileHandle(forWritingTo: fileURL) {
                defer { try? handle.close() }
                let size = (try? handle.seekToEnd()) ?? 0
                if size > maxBytes {
                    try? handle.truncate(atOffset: 0)
                }
                try? handle.write(contentsOf: data)
            } else {
                try? data.write(to: fileURL)
            }
        }
    }
}
