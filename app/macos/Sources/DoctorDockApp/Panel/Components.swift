import SwiftUI

/// The circular score indicator. Used at three sizes: popover header, panel
/// header, and nothing else — the menubar draws its own so it can stay flat.
struct ScoreBadge: View {
    let score: Int
    var size: CGFloat = 56

    var body: some View {
        ZStack {
            Circle()
                .stroke(Color.secondary.opacity(0.18), lineWidth: size * 0.09)
            Circle()
                .trim(from: 0, to: CGFloat(score) / 100)
                .stroke(scoreColor(score),
                        style: StrokeStyle(lineWidth: size * 0.09, lineCap: .round))
                .rotationEffect(.degrees(-90))
                .animation(.easeOut(duration: 0.4), value: score)

            Text("\(score)")
                .font(.system(size: size * 0.34, weight: .semibold, design: .rounded))
                .monospacedDigit()
                .foregroundStyle(scoreColor(score))
        }
        .frame(width: size, height: size)
        .accessibilityLabel("Health score \(score) out of 100, \(Format.grade(score))")
    }
}

/// The per-severity counts, as coloured pills.
struct SeverityTally: View {
    let counts: SeverityCounts

    var body: some View {
        HStack(spacing: 4) {
            ForEach(Severity.allCases.reversed(), id: \.self) { severity in
                let n = counts.count(severity)
                if n > 0 {
                    Text("\(n)")
                        .font(.system(size: 10, weight: .semibold))
                        .monospacedDigit()
                        .padding(.horizontal, 5)
                        .padding(.vertical, 1)
                        .background(severity.color.opacity(0.16), in: Capsule())
                        .foregroundStyle(severity.color)
                        .help("\(n) \(severity.rawValue.lowercased())")
                }
            }
        }
    }
}

/// One grouped finding. `compact` is the popover form.
struct FindingGroupRow: View {
    let group: FindingGroup
    var compact = false

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: group.severity.symbol)
                .foregroundStyle(group.severity.color)
                .font(.system(size: compact ? 11 : 13))
                .frame(width: 16)
                .padding(.top, 1)

            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Text(group.rule)
                        .font(compact ? .callout : .body)
                        .fontWeight(.medium)
                        .lineLimit(1)
                    Text(group.ruleID)
                        .font(.system(size: 9, design: .monospaced))
                        .foregroundStyle(.tertiary)
                }
                Text(subtitle)
                    .font(compact ? .caption2 : .caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(compact ? 1 : 2)
            }

            Spacer(minLength: 4)

            if group.count > 1 {
                Text("\(group.count)")
                    .font(.system(size: 10, weight: .semibold))
                    .monospacedDigit()
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 5)
                    .padding(.vertical, 1)
                    .background(Color.secondary.opacity(0.12), in: Capsule())
            }
        }
        .padding(.horizontal, 14)
        .padding(.vertical, compact ? 5 : 7)
        .contentShape(Rectangle())
    }

    private var subtitle: String {
        if group.count == 1 { return group.findings[0].resourceName }
        let shown = group.resourceNames.prefix(3).joined(separator: ", ")
        return group.count > 3 ? "\(shown) and \(group.count - 3) more" : shown
    }
}

/// Shown when a scan could not run. Docker being off is treated as a state
/// rather than an error, because on a laptop it usually is.
struct FailureView: View {
    let failure: DoctorDockCLI.Failure
    let retry: () -> Void

    var body: some View {
        VStack(spacing: 10) {
            Image(systemName: failure.isDockerDown ? "moon.zzz.fill" : "exclamationmark.triangle.fill")
                .font(.system(size: 28))
                .foregroundStyle(failure.isDockerDown ? Color.secondary : Color.orange)

            Text(failure.isDockerDown ? "Docker is not running" : "Could not scan")
                .font(.headline)

            Text(failure.errorDescription ?? "Unknown error")
                .font(.caption)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)

            if failure.isDockerDown {
                Button("Open Docker Desktop") {
                    NSWorkspace.shared.open(URL(fileURLWithPath: "/Applications/Docker.app"))
                }
                .controlSize(.small)
            }

            Button("Try again", action: retry)
                .controlSize(.small)
        }
        .frame(maxWidth: .infinity)
    }
}

/// A label/value stat used across the panel.
struct StatTile: View {
    let title: String
    let value: String
    var note: String?
    var tint: Color?

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(title.uppercased())
                .font(.system(size: 9, weight: .semibold))
                .foregroundStyle(.secondary)
            Text(value)
                .font(.system(size: 20, weight: .semibold, design: .rounded))
                .monospacedDigit()
                .foregroundStyle(tint ?? .primary)
            if let note {
                Text(note)
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
                    .lineLimit(1)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(10)
        .background(Color.secondary.opacity(0.06), in: RoundedRectangle(cornerRadius: 8))
    }
}
