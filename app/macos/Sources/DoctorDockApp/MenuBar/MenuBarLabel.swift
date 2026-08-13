import SwiftUI

/// What sits in the menubar: a status dot and the score.
///
/// The colour is the point. Most of the time nobody reads the number — they
/// notice that the dot went from green to red, which is the only thing an
/// always-visible indicator can usefully do.
struct MenuBarLabel: View {
    @ObservedObject var store: ScanStore

    var body: some View {
        HStack(spacing: 3) {
            Image(systemName: symbol)
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(tint)
            if let score = store.score {
                Text("\(score)")
                    .font(.system(size: 11, weight: .medium, design: .rounded))
                    .monospacedDigit()
            }
        }
        .help(tooltip)
    }

    private var symbol: String {
        switch store.state {
        case .scanning where store.report == nil:
            return "stethoscope"
        case .failed(let failure):
            // Docker being off is a normal state, not an error to alarm about.
            return failure.isDockerDown ? "moon.zzz" : "exclamationmark.triangle"
        default:
            guard let score = store.score else { return "stethoscope" }
            switch score {
            case 75...: return "checkmark.circle.fill"
            case 50..<75: return "exclamationmark.circle.fill"
            default: return "exclamationmark.triangle.fill"
            }
        }
    }

    private var tint: Color {
        switch store.state {
        case .failed(let failure):
            return failure.isDockerDown ? .secondary : .orange
        default:
            guard let score = store.score else { return .secondary }
            return scoreColor(score)
        }
    }

    private var tooltip: String {
        switch store.state {
        case .scanning where store.report == nil:
            return "DoctorDock — scanning…"
        case .failed(let failure):
            return failure.isDockerDown ? "Docker is not running" : (failure.errorDescription ?? "Scan failed")
        default:
            guard let score = store.score, let report = store.report else {
                return "DoctorDock"
            }
            let findings = report.summary.findings.total
            return "Docker health \(score)/100 — \(findings) finding\(findings == 1 ? "" : "s")"
        }
    }
}
