import SwiftUI
import AppKit

/// The sheet that turns the current scan into a shareable image.
///
/// It renders the card from summary numbers only — no names, tags, ports or
/// paths ever reach it — and lets the user save, copy or share the PNG. The
/// privacy note is shown on the sheet itself, because the whole point is that
/// this is safe to post in public.
struct ShareSheet: View {
    let report: Report
    @Environment(\.dismiss) private var dismiss

    @AppStorage("shareShowsPlatform") private var showPlatform = false
    @State private var savedURL: URL?

    private var card: ShareCard {
        ShareCard(score: report.score, summary: report.summary,
                  showPlatform: showPlatform,
                  platform: report.docker.operatingSystem ?? "")
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            preview
            Divider()
            controls
        }
        .frame(width: 560)
    }

    private var header: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text("Share your Docker health")
                    .font(.headline)
                Text("Only the numbers below leave your machine — no names, ports or paths.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            Button {
                dismiss()
            } label: {
                Image(systemName: "xmark.circle.fill")
                    .foregroundStyle(.secondary)
                    .font(.title2)
            }
            .buttonStyle(.plain)
        }
        .padding(16)
    }

    private var preview: some View {
        // The live card scaled to fit, so the preview is exactly the export.
        card
            .frame(width: ShareCard.size.width, height: ShareCard.size.height)
            .scaleEffect(496 / ShareCard.size.width)
            .frame(width: 496, height: 496)
            .clipShape(RoundedRectangle(cornerRadius: 16))
            .overlay(RoundedRectangle(cornerRadius: 16).strokeBorder(.white.opacity(0.08)))
            .padding(20)
    }

    private var controls: some View {
        VStack(spacing: 12) {
            Toggle("Include the Docker platform name (Docker Desktop)", isOn: $showPlatform)
                .toggleStyle(.checkbox)
                .font(.callout)
                .frame(maxWidth: .infinity, alignment: .leading)

            HStack(spacing: 10) {
                Button {
                    copy()
                } label: {
                    Label("Copy", systemImage: "doc.on.doc")
                        .frame(maxWidth: .infinity)
                }

                Button {
                    save()
                } label: {
                    Label("Save…", systemImage: "square.and.arrow.down")
                        .frame(maxWidth: .infinity)
                }

                ShareLinkButton(makeImage: renderNSImage)
                    .frame(maxWidth: .infinity)
            }
            .controlSize(.large)

            if let savedURL {
                Text("Saved to \(savedURL.path)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
        }
        .padding(16)
    }

    // MARK: - Export

    @MainActor
    private func renderNSImage() -> NSImage? {
        guard let data = CardRenderer.png(card, scale: 2),
              let image = NSImage(data: data) else { return nil }
        return image
    }

    private func copy() {
        guard let image = renderNSImage() else { return }
        NSPasteboard.general.clearContents()
        NSPasteboard.general.writeObjects([image])
    }

    private func save() {
        guard let data = CardRenderer.png(card, scale: 2) else { return }
        let panel = NSSavePanel()
        panel.allowedContentTypes = [.png]
        panel.nameFieldStringValue = "docker-health-\(report.score).png"
        panel.canCreateDirectories = true
        if panel.runModal() == .OK, let url = panel.url {
            try? data.write(to: url)
            savedURL = url
        }
    }
}

/// Wraps NSSharingServicePicker, which SwiftUI's ShareLink cannot drive for a
/// freshly-rendered NSImage on macOS as cleanly as the AppKit picker.
private struct ShareLinkButton: NSViewRepresentable {
    let makeImage: () -> NSImage?

    func makeNSView(context: Context) -> NSButton {
        let button = NSButton(title: "Share…", target: context.coordinator,
                              action: #selector(Coordinator.share(_:)))
        button.bezelStyle = .rounded
        button.image = NSImage(systemSymbolName: "square.and.arrow.up", accessibilityDescription: "Share")
        button.imagePosition = .imageLeading
        return button
    }

    func updateNSView(_ nsView: NSButton, context: Context) {
        context.coordinator.makeImage = makeImage
    }

    func makeCoordinator() -> Coordinator { Coordinator(makeImage: makeImage) }

    final class Coordinator: NSObject {
        var makeImage: () -> NSImage?
        init(makeImage: @escaping () -> NSImage?) { self.makeImage = makeImage }

        @objc func share(_ sender: NSButton) {
            guard let image = makeImage() else { return }
            let picker = NSSharingServicePicker(items: [image])
            picker.show(relativeTo: sender.bounds, of: sender, preferredEdge: .minY)
        }
    }
}


/// Window content for the Share scene: shows the card for the current scan, or
/// a prompt to scan first.
struct ShareWindowView: View {
    @EnvironmentObject private var store: ScanStore

    var body: some View {
        Group {
            if let report = store.report {
                ShareSheet(report: report)
            } else {
                VStack(spacing: 10) {
                    Image(systemName: "photo.on.rectangle.angled")
                        .font(.system(size: 30))
                        .foregroundStyle(.secondary)
                    Text("Nothing to share yet")
                        .font(.headline)
                    Text("Run a scan first, then come back.")
                        .font(.callout)
                        .foregroundStyle(.secondary)
                }
                .frame(width: 360, height: 220)
            }
        }
    }
}
