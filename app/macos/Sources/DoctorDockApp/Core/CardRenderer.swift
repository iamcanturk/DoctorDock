import SwiftUI
import AppKit

/// Rasterises a SwiftUI view to PNG.
///
/// Used both to produce the shareable card and, in `--render` mode, to dump the
/// app's views to disk so their design can be reviewed without a screen.
@MainActor
enum CardRenderer {

    /// Renders a view to PNG at the given pixel scale using ImageRenderer.
    ///
    /// ImageRenderer is right for the share card — a flat, fixed-size view — but
    /// it does not lay out ScrollView, List, Table or split views. For those use
    /// `hosted(_:size:)`, which drives a real AppKit view hierarchy.
    static func png<V: View>(_ view: V, scale: CGFloat = 2) -> Data? {
        let renderer = ImageRenderer(content: view)
        renderer.scale = scale
        renderer.isOpaque = true

        guard let nsImage = renderer.nsImage,
              let tiff = nsImage.tiffRepresentation,
              let rep = NSBitmapImageRep(data: tiff),
              let png = rep.representation(using: .png, properties: [:]) else {
            return nil
        }
        return png
    }

    /// Renders a view to PNG by hosting it in a real NSHostingView and capturing
    /// its layer. This lays out ScrollView/List/Table content, which
    /// ImageRenderer does not.
    static func hosted<V: View>(_ view: V, size: CGSize, settle: TimeInterval = 0) -> Data? {
        let host = NSHostingView(rootView: view)
        host.frame = CGRect(origin: .zero, size: size)
        host.layoutSubtreeIfNeeded()

        // Some views load content in a .task (the finding detail fetches its
        // explanation). Spinning the run loop lets that finish before capture,
        // so the render shows the real state rather than a spinner.
        if settle > 0 {
            let deadline = Date().addingTimeInterval(settle)
            while Date() < deadline {
                RunLoop.current.run(mode: .default, before: Date().addingTimeInterval(0.05))
            }
            host.layoutSubtreeIfNeeded()
        }

        guard let rep = host.bitmapImageRepForCachingDisplay(in: host.bounds) else {
            return nil
        }
        host.cacheDisplay(in: host.bounds, to: rep)
        return rep.representation(using: .png, properties: [:])
    }

    /// The share card as PNG, from summary numbers only.
    static func shareCard(score: Int, summary: Summary, platform: String?) -> Data? {
        png(ShareCard(score: score, summary: summary,
                      showPlatform: platform != nil,
                      platform: platform ?? ""))
    }
}
