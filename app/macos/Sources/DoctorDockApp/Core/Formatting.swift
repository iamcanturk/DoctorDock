import Foundation
import SwiftUI

/// Presentation helpers shared by every view.
///
/// Byte and duration formatting deliberately mirror the core's FormatBytes and
/// FormatDuration: a user comparing the app to the CLI must not see two
/// different numbers for the same thing.
enum Format {

    /// Decimal units with one decimal place, the way Docker reports sizes.
    static func bytes(_ value: Int64) -> String {
        guard value >= 0 else { return "unknown" }
        let unit: Int64 = 1000
        if value < unit { return "\(value) B" }

        var divisor = unit
        var exponent = 0
        var n = value / unit
        while n >= unit && exponent < 4 {
            divisor *= unit
            exponent += 1
            n /= unit
        }
        let symbols = ["k", "M", "G", "T", "P"]
        return String(format: "%.1f %@B", Double(value) / Double(divisor), symbols[exponent])
    }

    static func age(_ date: Date, now: Date = Date()) -> String {
        let seconds = now.timeIntervalSince(date)
        switch seconds {
        case ..<60: return "less than a minute"
        case ..<3600: return plural(Int(seconds / 60), "minute")
        case ..<86_400: return plural(Int(seconds / 3600), "hour")
        case ..<31_536_000: return plural(Int((seconds / 86_400).rounded()), "day")
        default: return plural(Int((seconds / 31_536_000).rounded()), "year")
        }
    }

    static func relative(_ date: Date) -> String {
        let seconds = Date().timeIntervalSince(date)
        if seconds < 10 { return "just now" }
        if seconds < 60 { return "\(Int(seconds))s ago" }
        return "\(age(date)) ago"
    }

    private static func plural(_ n: Int, _ noun: String) -> String {
        n == 1 ? "1 \(noun)" : "\(n) \(noun)s"
    }

    /// The label the CLI uses for a score, so the two agree.
    static func grade(_ score: Int) -> String {
        switch score {
        case 90...: return "excellent"
        case 75..<90: return "good"
        case 50..<75: return "needs attention"
        case 25..<50: return "poor"
        default: return "critical"
        }
    }
}

extension Severity {
    var color: Color {
        switch self {
        case .critical: return Color(red: 0.83, green: 0.13, blue: 0.13)
        case .high: return Color(red: 0.90, green: 0.35, blue: 0.13)
        case .medium: return Color(red: 0.85, green: 0.62, blue: 0.10)
        case .low: return Color(red: 0.20, green: 0.50, blue: 0.80)
        case .info: return Color.secondary
        }
    }

    var symbol: String {
        switch self {
        case .critical: return "exclamationmark.octagon.fill"
        case .high: return "exclamationmark.triangle.fill"
        case .medium: return "exclamationmark.circle.fill"
        case .low: return "info.circle.fill"
        case .info: return "circle.fill"
        }
    }
}

extension Risk {
    var color: Color {
        switch self {
        case .safe: return Color(red: 0.20, green: 0.60, blue: 0.30)
        case .review: return Color(red: 0.85, green: 0.62, blue: 0.10)
        case .dataLoss: return Color(red: 0.83, green: 0.13, blue: 0.13)
        case .unknown: return .secondary
        }
    }

    var symbol: String {
        switch self {
        case .safe: return "checkmark.circle.fill"
        case .review: return "questionmark.circle.fill"
        case .dataLoss: return "exclamationmark.octagon.fill"
        case .unknown: return "circle"
        }
    }
}

extension ResourceKind {
    var symbol: String {
        switch self {
        case .container: return "shippingbox"
        case .image: return "square.stack.3d.up"
        case .volume: return "externaldrive"
        case .network: return "network"
        case .system, .unknown: return "gearshape"
        }
    }

    var plural: String {
        switch self {
        case .container: return "Containers"
        case .image: return "Images"
        case .volume: return "Volumes"
        case .network: return "Networks"
        case .system, .unknown: return "System"
        }
    }
}

extension Color {
    /// The Docker-blue brand accent, used for chrome — buttons, selection, the
    /// logo. Score and severity keep their semantic colours (green good, red
    /// bad); the brand colour is for identity, not meaning.
    static let brand = Color(red: 0.14, green: 0.588, blue: 0.929)      // #2496ED
    static let brandDeep = Color(red: 0.043, green: 0.42, blue: 0.77)
}

/// Maps a score onto the colour used everywhere it appears, including the
/// menubar icon.
func scoreColor(_ score: Int) -> Color {
    switch score {
    case 75...: return Color(red: 0.20, green: 0.65, blue: 0.32)
    case 50..<75: return Color(red: 0.85, green: 0.62, blue: 0.10)
    default: return Color(red: 0.83, green: 0.20, blue: 0.15)
    }
}
