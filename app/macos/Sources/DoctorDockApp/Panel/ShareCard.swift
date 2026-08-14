import SwiftUI
#if canImport(AppKit)
import AppKit
#endif

/// The shareable "here is my Docker" card.
///
/// This is the one view whose privacy matters most, because it is made to be
/// posted in public. It shows ONLY aggregate numbers: the score, resource
/// counts and per-severity finding counts. It never shows a container name, an
/// image tag, a port, a path, an environment key, or the daemon's host name.
/// Everything it can render comes from `Summary` and the score — the resource
/// lists are not even passed in.
///
/// The canvas is 16:9. That is the size X, LinkedIn and most feeds display a
/// single image at without cropping, and it doubles as the site's link-preview
/// (og:image / twitter:image) card.
struct ShareCard: View {
    let score: Int
    let summary: Summary
    /// Whether to name Docker Desktop / the platform. Off by default.
    var showPlatform = false
    var platform = ""

    /// The canvas size. 16:9 reads best on X and as a link preview.
    static let size = CGSize(width: 1600, height: 900)

    private var accent: Color { scoreColor(score) }

    var body: some View {
        ZStack {
            background
            VStack(spacing: 0) {
                header
                Spacer(minLength: 20)
                HStack(spacing: 68) {
                    hero
                    metrics
                }
                Spacer(minLength: 20)
                severityRow
                Spacer(minLength: 20)
                footer
            }
            .padding(.horizontal, 78)
            .padding(.vertical, 64)
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
                startPoint: .topLeading, endPoint: .bottomTrailing)

            // A faint dot grid for texture — reads as "developer tool" without
            // competing with the content.
            DotGrid(spacing: 36, dotSize: 2)
                .foregroundStyle(.white.opacity(0.03))

            // A wash of the brand colour behind the score ring — the card is
            // brand-first; the score's own colour lives in the ring itself.
            RadialGradient(
                colors: [Color.brand.opacity(0.22), .clear],
                center: .init(x: 0.24, y: 0.52), startRadius: 20, endRadius: 560)
        }
    }

    // MARK: - Header

    private var header: some View {
        HStack(spacing: 18) {
            logo
            VStack(alignment: .leading, spacing: 2) {
                Text("DoctorDock")
                    .font(.system(size: 42, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                Text("Docker health report")
                    .font(.system(size: 20, weight: .medium))
                    .foregroundStyle(.white.opacity(0.5))
            }
            Spacer()
            if showPlatform, !platform.isEmpty {
                Text(platform)
                    .font(.system(size: 17, weight: .medium))
                    .foregroundStyle(.white.opacity(0.42))
                    .padding(.horizontal, 16).padding(.vertical, 9)
                    .background(.white.opacity(0.05), in: Capsule())
                    .overlay(Capsule().strokeBorder(.white.opacity(0.06)))
            }
        }
    }

    /// The mascot logo, on a soft brand-tinted disc. Falls back to a glyph in
    /// contexts where the bundle has no copy of the image (a bare `swift run`).
    private var logo: some View {
        ZStack {
            Circle()
                .fill(Color.brand.opacity(0.16))
                .frame(width: 96, height: 96)
            Circle()
                .strokeBorder(Color.brand.opacity(0.30), lineWidth: 1.5)
                .frame(width: 96, height: 96)
            if let mascot = BrandAsset.mascot {
                mascot
                    .resizable()
                    .scaledToFit()
                    .frame(width: 82, height: 82)
                    .shadow(color: .black.opacity(0.35), radius: 6, y: 2)
            } else {
                Image(systemName: "stethoscope")
                    .font(.system(size: 44, weight: .semibold))
                    .foregroundStyle(Color.brand)
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
            VStack(spacing: -6) {
                Text("\(score)")
                    .font(.system(size: 152, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                    .monospacedDigit()
                Text(Format.grade(score).uppercased())
                    .font(.system(size: 22, weight: .heavy, design: .rounded))
                    .tracking(5)
                    .foregroundStyle(accent)
                    .padding(.top, 10)
                Text("out of 100")
                    .font(.system(size: 15, weight: .medium))
                    .foregroundStyle(.white.opacity(0.35))
                    .padding(.top, 2)
            }
        }
        .frame(width: 360, height: 360)
    }

    // MARK: - Metrics

    private var metrics: some View {
        Grid(horizontalSpacing: 18, verticalSpacing: 18) {
            GridRow {
                metric("\(summary.containers.total)", "containers",
                       sub: "\(summary.containers.running) running")
                metric("\(summary.images.total)", "images",
                       sub: Format.bytes(summary.images.totalSize))
            }
            GridRow {
                metric("\(summary.volumes.total)", "volumes",
                       sub: "\(summary.volumes.unused) unused")
                metric("\(summary.networks.custom)", "networks",
                       sub: "\(summary.networks.unused) unused")
            }
        }
        .frame(maxWidth: .infinity)
    }

    private func metric(_ value: String, _ label: String, sub: String) -> some View {
        VStack(spacing: 6) {
            Text(value)
                .font(.system(size: 58, weight: .bold, design: .rounded))
                .foregroundStyle(.white)
                .monospacedDigit()
            Text(label)
                .font(.system(size: 19, weight: .semibold))
                .foregroundStyle(.white.opacity(0.72))
            Text(sub)
                .font(.system(size: 14, weight: .medium))
                .foregroundStyle(.white.opacity(0.42))
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 30)
        .background(.white.opacity(0.035), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous)
            .strokeBorder(.white.opacity(0.05)))
    }

    // MARK: - Severity

    private var severityRow: some View {
        HStack(spacing: 12) {
            ForEach(Severity.allCases.reversed(), id: \.self) { severity in
                let n = summary.findings.bySeverity.count(severity)
                HStack(spacing: 9) {
                    Circle().fill(severity.color).frame(width: 12, height: 12)
                    Text("\(n)")
                        .font(.system(size: 23, weight: .bold, design: .rounded))
                        .monospacedDigit().foregroundStyle(.white)
                    Text(severity.rawValue.lowercased())
                        .font(.system(size: 16, weight: .medium))
                        .foregroundStyle(.white.opacity(0.5))
                }
                .padding(.horizontal, 22).padding(.vertical, 14)
                .frame(maxWidth: .infinity)
                .background(.white.opacity(n > 0 ? 0.05 : 0.02), in: Capsule())
                .overlay(Capsule().strokeBorder(.white.opacity(n > 0 ? 0.05 : 0)))
                .opacity(n > 0 ? 1 : 0.4)
            }
        }
    }

    // MARK: - Footer

    private var footer: some View {
        HStack(spacing: 9) {
            ForEach(["No AI", "Runs offline", "No data collected"], id: \.self) { badge in
                HStack(spacing: 7) {
                    Image(systemName: icon(badge)).font(.system(size: 14))
                    Text(badge).font(.system(size: 16, weight: .semibold))
                }
                .foregroundStyle(.white.opacity(0.58))
                .padding(.horizontal, 16).padding(.vertical, 9)
                .background(.white.opacity(0.04), in: Capsule())
            }
            Spacer()
            Text("doctordock.iamcanturk.dev")
                .font(.system(size: 17, weight: .semibold, design: .monospaced))
                .foregroundStyle(Color.brand.opacity(0.85))
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

/// The mascot logo, loaded from the app bundle once. Nil in contexts where the
/// bundle has no copy (for example a bare `swift run`); callers fall back to a
/// glyph so the card still renders.
enum BrandAsset {
    static let mascot: Image? = {
        #if canImport(AppKit)
        if let url = Bundle.main.url(forResource: "mascot", withExtension: "png"),
           let image = NSImage(contentsOf: url) {
            return Image(nsImage: image)
        }
        #endif
        return nil
    }()
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
