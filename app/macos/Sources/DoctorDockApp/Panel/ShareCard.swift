import SwiftUI

/// The shareable "here is my Docker" card.
///
/// This is the one view whose privacy matters most, because it is made to be
/// posted in public. It shows ONLY aggregate numbers: the score, resource
/// counts and per-severity finding counts. It never shows a container name, an
/// image tag, a port, a path, an environment key, or the daemon's host name.
/// Everything it can render comes from `Summary` and the score — the resource
/// lists are not even passed in.
///
/// It is a fixed-size view designed to look right as a 2x PNG, not to adapt to
/// a window.
struct ShareCard: View {
    let score: Int
    let summary: Summary
    /// Whether to name Docker Desktop / the platform. Off by default — even the
    /// daemon flavour is arguably nobody's business on a public post.
    var showPlatform = false
    var platform = ""

    /// The canvas size. Square reads well on every network.
    static let size = CGSize(width: 1080, height: 1080)

    var body: some View {
        ZStack {
            background

            VStack(spacing: 0) {
                header
                Spacer(minLength: 0)
                scoreBlock
                Spacer(minLength: 0)
                resourceRow
                    .padding(.top, 44)
                findingsRow
                    .padding(.top, 28)
                Spacer(minLength: 0)
                footer
            }
            .padding(72)
        }
        .frame(width: Self.size.width, height: Self.size.height)
    }

    // MARK: - Background

    private var background: some View {
        ZStack {
            LinearGradient(
                colors: [Color(red: 0.07, green: 0.09, blue: 0.13),
                         Color(red: 0.04, green: 0.05, blue: 0.08)],
                startPoint: .topLeading, endPoint: .bottomTrailing)

            // A soft wash of the score colour, so a healthy card feels green and
            // a bad one feels red before a single number is read.
            RadialGradient(
                colors: [accent.opacity(0.20), .clear],
                center: .center, startRadius: 40, endRadius: 620)
        }
    }

    private var accent: Color { scoreColor(score) }

    // MARK: - Header

    private var header: some View {
        HStack(spacing: 14) {
            Image(systemName: "stethoscope")
                .font(.system(size: 34, weight: .medium))
                .foregroundStyle(accent)
            VStack(alignment: .leading, spacing: 0) {
                Text("DoctorDock")
                    .font(.system(size: 34, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                Text("Docker health report")
                    .font(.system(size: 18, weight: .medium))
                    .foregroundStyle(.white.opacity(0.5))
            }
            Spacer()
            if showPlatform, !platform.isEmpty {
                Text(platform)
                    .font(.system(size: 16, weight: .medium))
                    .foregroundStyle(.white.opacity(0.4))
            }
        }
    }

    // MARK: - Score

    private var scoreBlock: some View {
        VStack(spacing: 18) {
            ZStack {
                Circle()
                    .stroke(Color.white.opacity(0.08), lineWidth: 26)
                Circle()
                    .trim(from: 0, to: CGFloat(score) / 100)
                    .stroke(accent, style: StrokeStyle(lineWidth: 26, lineCap: .round))
                    .rotationEffect(.degrees(-90))
                    .shadow(color: accent.opacity(0.5), radius: 24)

                VStack(spacing: -6) {
                    Text("\(score)")
                        .font(.system(size: 150, weight: .bold, design: .rounded))
                        .foregroundStyle(.white)
                        .monospacedDigit()
                    Text("/ 100")
                        .font(.system(size: 30, weight: .medium, design: .rounded))
                        .foregroundStyle(.white.opacity(0.4))
                }
            }
            .frame(width: 340, height: 340)

            Text(Format.grade(score).uppercased())
                .font(.system(size: 24, weight: .heavy, design: .rounded))
                .tracking(4)
                .foregroundStyle(accent)
        }
    }

    // MARK: - Resources

    private var resourceRow: some View {
        HStack(spacing: 0) {
            metric("\(summary.containers.total)", "containers",
                   sub: "\(summary.containers.running) running")
            divider
            metric("\(summary.images.total)", "images",
                   sub: Format.bytes(summary.images.totalSize))
            divider
            metric("\(summary.volumes.total)", "volumes",
                   sub: "\(summary.volumes.unused) unused")
            divider
            metric("\(summary.networks.custom)", "networks",
                   sub: "\(summary.networks.unused) unused")
        }
    }

    private func metric(_ value: String, _ label: String, sub: String) -> some View {
        VStack(spacing: 4) {
            Text(value)
                .font(.system(size: 52, weight: .bold, design: .rounded))
                .foregroundStyle(.white)
                .monospacedDigit()
            Text(label)
                .font(.system(size: 18, weight: .semibold))
                .foregroundStyle(.white.opacity(0.7))
            Text(sub)
                .font(.system(size: 14, weight: .medium))
                .foregroundStyle(.white.opacity(0.4))
        }
        .frame(maxWidth: .infinity)
    }

    private var divider: some View {
        Rectangle()
            .fill(Color.white.opacity(0.08))
            .frame(width: 1, height: 64)
    }

    // MARK: - Findings

    private var findingsRow: some View {
        HStack(spacing: 12) {
            ForEach(Severity.allCases.reversed(), id: \.self) { severity in
                let n = summary.findings.bySeverity.count(severity)
                pill(count: n, severity: severity)
            }
        }
    }

    private func pill(count: Int, severity: Severity) -> some View {
        HStack(spacing: 8) {
            Circle().fill(severity.color).frame(width: 12, height: 12)
            Text("\(count)")
                .font(.system(size: 22, weight: .bold, design: .rounded))
                .monospacedDigit()
                .foregroundStyle(.white)
            Text(severity.rawValue.lowercased())
                .font(.system(size: 16, weight: .medium))
                .foregroundStyle(.white.opacity(0.55))
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 12)
        .background(Color.white.opacity(count > 0 ? 0.06 : 0.02),
                    in: Capsule())
        .opacity(count > 0 ? 1 : 0.45)
    }

    // MARK: - Footer

    private var footer: some View {
        HStack(spacing: 10) {
            Label("No AI", systemImage: "cpu")
            dot
            Label("Runs offline", systemImage: "wifi.slash")
            dot
            Label("No data collected", systemImage: "lock.shield")
            Spacer()
            Text("github.com/iamcanturk/DoctorDock")
                .foregroundStyle(.white.opacity(0.35))
        }
        .font(.system(size: 15, weight: .medium))
        .foregroundStyle(.white.opacity(0.5))
        .labelStyle(.titleAndIcon)
    }

    private var dot: some View {
        Circle().fill(Color.white.opacity(0.2)).frame(width: 4, height: 4)
    }
}
