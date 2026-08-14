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
/// (og:image / twitter:image) card. The composition is deliberately poster-like:
/// the mascot is a hero element on the left, the numbers a clean panel on the
/// right, over a layered brand-blue backdrop.
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
                topBar
                Spacer(minLength: 0)
                HStack(alignment: .center, spacing: 60) {
                    mascotPoster
                        .frame(width: 452)
                    stats
                        .frame(maxWidth: .infinity)
                }
                Spacer(minLength: 0)
                severityRow
                Spacer(minLength: 22)
                footer
            }
            .padding(.horizontal, 72)
            .padding(.vertical, 58)
        }
        .frame(width: Self.size.width, height: Self.size.height)
        .environment(\.colorScheme, .dark)
    }

    // MARK: - Background

    private var background: some View {
        ZStack {
            // Base: a deep navy, lit from the top-left.
            LinearGradient(
                colors: [Color(red: 0.055, green: 0.095, blue: 0.17),
                         Color(red: 0.02, green: 0.03, blue: 0.06)],
                startPoint: .topLeading, endPoint: .bottomTrailing)

            // A big brand bloom behind the mascot, and a cooler one bottom-right,
            // so the card has depth and a light source rather than a flat wash.
            RadialGradient(
                colors: [Color.brand.opacity(0.38), .clear],
                center: .init(x: 0.23, y: 0.44), startRadius: 20, endRadius: 540)
            RadialGradient(
                colors: [Color(red: 0.13, green: 0.34, blue: 0.72).opacity(0.32), .clear],
                center: .init(x: 0.92, y: 0.9), startRadius: 20, endRadius: 620)

            // Faint outlined cubes — the same "developer tool" motif as the site,
            // placed for texture without competing with the content.
            CubeField()

            // A soft top sheen and a vignette to focus the centre.
            LinearGradient(colors: [.white.opacity(0.05), .clear],
                           startPoint: .top, endPoint: .center)
                .blendMode(.plusLighter)
            RadialGradient(colors: [.clear, .black.opacity(0.38)],
                           center: .center, startRadius: 380, endRadius: 1020)
        }
    }

    // MARK: - Top bar

    private var topBar: some View {
        HStack(spacing: 10) {
            Text("DOCKER HEALTH REPORT")
                .font(.system(size: 15, weight: .heavy, design: .rounded))
                .tracking(3)
                .foregroundStyle(Color.brand.opacity(0.9))
            Spacer()
            if showPlatform, !platform.isEmpty {
                Text(platform)
                    .font(.system(size: 16, weight: .medium))
                    .foregroundStyle(.white.opacity(0.44))
                    .padding(.horizontal, 16).padding(.vertical, 9)
                    .background(.white.opacity(0.05), in: Capsule())
                    .overlay(Capsule().strokeBorder(.white.opacity(0.07)))
            }
        }
    }

    // MARK: - Mascot poster (the hero)

    private var mascotPoster: some View {
        VStack(spacing: 22) {
            ZStack {
                // Glow disc behind the mascot.
                Circle()
                    .fill(RadialGradient(
                        colors: [Color.brand.opacity(0.5), Color.brand.opacity(0.06), .clear],
                        center: .center, startRadius: 8, endRadius: 230))
                    .frame(width: 420, height: 420)
                    .blur(radius: 6)
                // A soft ground shadow, so the character sits in the scene.
                Ellipse()
                    .fill(.black.opacity(0.4))
                    .frame(width: 240, height: 36)
                    .offset(y: 158)
                    .blur(radius: 14)
                mascotImage
                    .frame(width: 320, height: 320)
                    .shadow(color: Color.brand.opacity(0.55), radius: 34)
                    .shadow(color: .black.opacity(0.45), radius: 12, y: 10)
            }
            .frame(height: 344)

            VStack(spacing: 4) {
                Text("DoctorDock")
                    .font(.system(size: 50, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                Text("a doctor for your Docker")
                    .font(.system(size: 19, weight: .medium))
                    .foregroundStyle(.white.opacity(0.55))
            }
        }
    }

    @ViewBuilder private var mascotImage: some View {
        if let mascot = BrandAsset.mascot {
            mascot.resizable().scaledToFit()
        } else {
            Image(systemName: "stethoscope")
                .font(.system(size: 150, weight: .semibold))
                .foregroundStyle(Color.brand)
        }
    }

    // MARK: - Stats panel

    private var stats: some View {
        HStack(spacing: 44) {
            hero
            metrics
        }
    }

    // MARK: - Hero score ring

    private var hero: some View {
        ZStack {
            Circle()
                .stroke(.white.opacity(0.08), lineWidth: 20)
            Circle()
                .trim(from: 0, to: CGFloat(score) / 100)
                .stroke(
                    AngularGradient(
                        colors: [accent.opacity(0.7), accent],
                        center: .center, startAngle: .degrees(-90), endAngle: .degrees(270)),
                    style: StrokeStyle(lineWidth: 20, lineCap: .round))
                .rotationEffect(.degrees(-90))
                .shadow(color: accent.opacity(0.6), radius: 24)
            VStack(spacing: -4) {
                Text("\(score)")
                    .font(.system(size: 128, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                    .monospacedDigit()
                Text(Format.grade(score).uppercased())
                    .font(.system(size: 20, weight: .heavy, design: .rounded))
                    .tracking(4)
                    .foregroundStyle(accent)
                    .padding(.top, 8)
                Text("out of 100")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(.white.opacity(0.35))
                    .padding(.top, 2)
            }
        }
        .frame(width: 296, height: 296)
    }

    // MARK: - Metrics

    private var metrics: some View {
        Grid(horizontalSpacing: 16, verticalSpacing: 16) {
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
        VStack(spacing: 5) {
            Text(value)
                .font(.system(size: 54, weight: .bold, design: .rounded))
                .foregroundStyle(.white)
                .monospacedDigit()
            Text(label)
                .font(.system(size: 18, weight: .semibold))
                .foregroundStyle(.white.opacity(0.72))
            Text(sub)
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(.white.opacity(0.42))
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 26)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.white.opacity(0.04))
                .background(
                    RoundedRectangle(cornerRadius: 18, style: .continuous)
                        .fill(Color.brand.opacity(0.05))))
        .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous)
            .strokeBorder(.white.opacity(0.06)))
    }

    // MARK: - Severity

    private var severityRow: some View {
        HStack(spacing: 12) {
            ForEach(Severity.allCases.reversed(), id: \.self) { severity in
                let n = summary.findings.bySeverity.count(severity)
                HStack(spacing: 9) {
                    Circle().fill(severity.color).frame(width: 12, height: 12)
                        .shadow(color: severity.color.opacity(n > 0 ? 0.7 : 0), radius: 5)
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
                .overlay(Capsule().strokeBorder(.white.opacity(n > 0 ? 0.06 : 0)))
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

/// A field of faint, outlined cubes — the "developer tool" motif shared with the
/// landing page. Positions are fixed, so the card renders identically every time.
struct CubeField: View {
    // x, y as a fraction of the canvas; side in points; rotation in degrees; opacity.
    private let cubes: [(CGFloat, CGFloat, CGFloat, Double, Double)] = [
        (0.07, 0.15, 50, 18, 0.10), (0.60, 0.10, 30, -14, 0.07),
        (0.90, 0.26, 58, 24, 0.09), (0.50, 0.86, 42, 8, 0.06),
        (0.15, 0.74, 36, -22, 0.08), (0.80, 0.64, 46, 14, 0.07),
        (0.34, 0.30, 24, 32, 0.06), (0.70, 0.92, 28, -8, 0.05),
        (0.44, 0.16, 20, 12, 0.05), (0.05, 0.46, 30, -6, 0.06),
    ]

    var body: some View {
        Canvas { context, size in
            for (fx, fy, side, rot, op) in cubes {
                let rect = CGRect(x: -side / 2, y: -side / 2, width: side, height: side)
                let transform = CGAffineTransform(translationX: fx * size.width, y: fy * size.height)
                    .rotated(by: rot * .pi / 180)
                let path = Path(roundedRect: rect, cornerRadius: 4).applying(transform)
                context.stroke(path,
                               with: .color(Color(red: 0.35, green: 0.66, blue: 0.94).opacity(op)),
                               lineWidth: 2)
            }
        }
    }
}
