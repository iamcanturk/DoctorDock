import SwiftUI

/// The shareable "here is my Docker" card.
///
/// This is the one view whose privacy matters most, because it is made to be
/// posted in public. It shows ONLY aggregate numbers: the score, resource
/// counts and per-severity finding counts. It never shows a container name, an
/// image tag, a port, a path, an environment key, or the daemon's host name.
/// Everything it can render comes from `Summary` and the score — the resource
/// lists are not even passed in.
struct ShareCard: View {
    let score: Int
    let summary: Summary
    /// Whether to name Docker Desktop / the platform. Off by default.
    var showPlatform = false
    var platform = ""

    /// The canvas size. Square reads well on every network.
    static let size = CGSize(width: 1080, height: 1080)

    private var accent: Color { scoreColor(score) }

    var body: some View {
        ZStack {
            background
            VStack(spacing: 0) {
                header
                Spacer(minLength: 0)
                hero
                Spacer(minLength: 0)
                metrics
                    .padding(.top, 40)
                severityRow
                    .padding(.top, 26)
                Spacer(minLength: 0)
                footer
            }
            .padding(76)
        }
        .frame(width: Self.size.width, height: Self.size.height)
        .environment(\.colorScheme, .dark)
    }

    // MARK: - Background

    private var background: some View {
        ZStack {
            LinearGradient(
                colors: [Color(red: 0.08, green: 0.10, blue: 0.15),
                         Color(red: 0.03, green: 0.04, blue: 0.07)],
                startPoint: .top, endPoint: .bottom)

            // A faint dot grid for texture — reads as "developer tool" without
            // competing with the content.
            DotGrid(spacing: 34, dotSize: 2)
                .foregroundStyle(.white.opacity(0.03))

            // A wash of the score colour behind the ring, so the card's mood
            // matches its number before anything is read.
            RadialGradient(
                colors: [accent.opacity(0.22), .clear],
                center: .init(x: 0.5, y: 0.42), startRadius: 30, endRadius: 560)
        }
    }

    // MARK: - Header

    private var header: some View {
        HStack(spacing: 14) {
            ZStack {
                RoundedRectangle(cornerRadius: 13, style: .continuous)
                    .fill(accent.opacity(0.16))
                    .frame(width: 58, height: 58)
                Image(systemName: "stethoscope")
                    .font(.system(size: 30, weight: .semibold))
                    .foregroundStyle(accent)
            }
            VStack(alignment: .leading, spacing: 1) {
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
                    .font(.system(size: 15, weight: .medium))
                    .foregroundStyle(.white.opacity(0.4))
                    .padding(.horizontal, 14).padding(.vertical, 8)
                    .background(.white.opacity(0.05), in: Capsule())
            }
        }
    }

    // MARK: - Hero score

    private var hero: some View {
        ZStack {
            Circle()
                .stroke(.white.opacity(0.07), lineWidth: 22)
            Circle()
                .trim(from: 0, to: CGFloat(score) / 100)
                .stroke(
                    AngularGradient(
                        colors: [accent.opacity(0.7), accent],
                        center: .center, startAngle: .degrees(-90), endAngle: .degrees(270)),
                    style: StrokeStyle(lineWidth: 22, lineCap: .round))
                .rotationEffect(.degrees(-90))
                .shadow(color: accent.opacity(0.55), radius: 26)
            VStack(spacing: -4) {
                Text("\(score)")
                    .font(.system(size: 168, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                    .monospacedDigit()
                Text(Format.grade(score).uppercased())
                    .font(.system(size: 22, weight: .heavy, design: .rounded))
                    .tracking(5)
                    .foregroundStyle(accent)
                    .padding(.top, 8)
            }
        }
        .frame(width: 380, height: 380)
    }

    // MARK: - Metrics

    private var metrics: some View {
        HStack(spacing: 14) {
            metric("\(summary.containers.total)", "containers",
                   sub: "\(summary.containers.running) running")
            metric("\(summary.images.total)", "images",
                   sub: Format.bytes(summary.images.totalSize))
            metric("\(summary.volumes.total)", "volumes",
                   sub: "\(summary.volumes.unused) unused")
            metric("\(summary.networks.custom)", "networks",
                   sub: "\(summary.networks.unused) unused")
        }
    }

    private func metric(_ value: String, _ label: String, sub: String) -> some View {
        VStack(spacing: 5) {
            Text(value)
                .font(.system(size: 50, weight: .bold, design: .rounded))
                .foregroundStyle(.white)
                .monospacedDigit()
            Text(label)
                .font(.system(size: 17, weight: .semibold))
                .foregroundStyle(.white.opacity(0.72))
            Text(sub)
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(.white.opacity(0.42))
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 22)
        .background(.white.opacity(0.035), in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous)
            .strokeBorder(.white.opacity(0.05)))
    }

    // MARK: - Severity

    private var severityRow: some View {
        HStack(spacing: 10) {
            ForEach(Severity.allCases.reversed(), id: \.self) { severity in
                let n = summary.findings.bySeverity.count(severity)
                HStack(spacing: 8) {
                    Circle().fill(severity.color).frame(width: 11, height: 11)
                    Text("\(n)")
                        .font(.system(size: 21, weight: .bold, design: .rounded))
                        .monospacedDigit().foregroundStyle(.white)
                    Text(severity.rawValue.lowercased())
                        .font(.system(size: 15, weight: .medium))
                        .foregroundStyle(.white.opacity(0.5))
                }
                .padding(.horizontal, 18).padding(.vertical, 12)
                .background(.white.opacity(n > 0 ? 0.05 : 0.02), in: Capsule())
                .opacity(n > 0 ? 1 : 0.4)
            }
        }
    }

    // MARK: - Footer

    private var footer: some View {
        HStack(spacing: 8) {
            ForEach(["No AI", "Runs offline", "No data collected"], id: \.self) { badge in
                HStack(spacing: 6) {
                    Image(systemName: icon(badge)).font(.system(size: 13))
                    Text(badge).font(.system(size: 15, weight: .semibold))
                }
                .foregroundStyle(.white.opacity(0.55))
                .padding(.horizontal, 14).padding(.vertical, 8)
                .background(.white.opacity(0.04), in: Capsule())
            }
            Spacer()
            Text("github.com/iamcanturk/DoctorDock")
                .font(.system(size: 15, weight: .medium, design: .monospaced))
                .foregroundStyle(.white.opacity(0.35))
        }
    }

    private func icon(_ badge: String) -> String {
        switch badge {
        case "No AI": return "cpu"
        case "Runs offline": return "wifi.slash"
        default: return "lock.shield"
        }
    }
}

/// A faint dot grid used behind the share card.
struct DotGrid: View {
    var spacing: CGFloat = 32
    var dotSize: CGFloat = 2

    var body: some View {
        Canvas { context, size in
            let dot = Path(ellipseIn: CGRect(x: 0, y: 0, width: dotSize, height: dotSize))
            var y: CGFloat = 0
            while y < size.height {
                var x: CGFloat = 0
                while x < size.width {
                    context.fill(dot.offsetBy(dx: x, dy: y), with: .color(.white))
                    x += spacing
                }
                y += spacing
            }
        }
    }
}
