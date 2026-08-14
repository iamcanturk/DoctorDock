import SwiftUI

/// The menubar dropdown: enough to decide whether to care, and a way into the
/// panel when you do.
struct PopoverView: View {
    @EnvironmentObject private var store: ScanStore
    @Environment(\.openWindow) private var openWindow
    @Environment(\.openSettings) private var openSettings

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header

            Divider()

            switch store.state {
            case .idle:
                idle
            case .scanning where store.report == nil:
                loading
            case .failed(let failure) where store.report == nil:
                FailureView(failure: failure) {
                    Task { await store.refresh() }
                }
                .padding(16)
            default:
                if let report = store.report {
                    content(report)
                }
            }

            Divider()
            footer
        }
        .frame(width: 360)
    }

    // MARK: - Header

    private var header: some View {
        HStack(alignment: .center, spacing: 10) {
            if let score = store.score {
                ScoreBadge(score: score, size: 44)
            } else {
                Image(systemName: "stethoscope")
                    .font(.system(size: 22))
                    .foregroundStyle(.secondary)
                    .frame(width: 44, height: 44)
            }

            VStack(alignment: .leading, spacing: 2) {
                Text("DoctorDock")
                    .font(.headline)
                if let report = store.report {
                    Text(report.docker.display)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                } else if case .failed(let failure) = store.state {
                    Text(failure.isDockerDown ? "Docker is not running" : "Scan failed")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }

            Spacer()

            Button {
                Task { await store.refresh() }
            } label: {
                Image(systemName: "arrow.clockwise")
                    .rotationEffect(.degrees(store.isScanning ? 360 : 0))
                    .animation(store.isScanning
                        ? .linear(duration: 1).repeatForever(autoreverses: false)
                        : .default,
                        value: store.isScanning)
            }
            .buttonStyle(.borderless)
            .help("Rescan now")
            .disabled(store.isScanning)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 12)
    }

    // A store that has not scanned is a different thing from one that is
    // scanning, and must not look the same — that is what made a missing
    // first scan read as a hung one.
    private var idle: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("No scan yet.")
                .font(.callout)
                .foregroundStyle(.secondary)
            Button("Scan now") {
                Task { await store.refresh() }
            }
            .controlSize(.small)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(16)
    }

    private var loading: some View {
        HStack(spacing: 8) {
            ProgressView().controlSize(.small)
            Text("Scanning your Docker environment…")
                .font(.callout)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(16)
    }

    // MARK: - Content

    private func content(_ report: Report) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            resourceRow(report)

            Divider().padding(.horizontal, 14)

            if report.findings.isEmpty {
                Label("No findings. This environment looks healthy.", systemImage: "checkmark.seal.fill")
                    .font(.callout)
                    .foregroundStyle(.green)
                    .padding(14)
            } else {
                findings(report)
            }

            if report.summary.images.reclaimableSize > 0 {
                Divider().padding(.horizontal, 14)
                reclaimRow(report)
            }
        }
    }

    private func resourceRow(_ report: Report) -> some View {
        HStack(spacing: 0) {
            stat("Containers",
                 "\(report.summary.containers.running)/\(report.summary.containers.total)",
                 "running")
            stat("Images", "\(report.summary.images.total)",
                 Format.bytes(report.summary.images.totalSize))
            stat("Volumes", "\(report.summary.volumes.total)",
                 "\(report.summary.volumes.unused) unused")
            stat("Networks", "\(report.summary.networks.custom)",
                 "\(report.summary.networks.unused) unused")
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 10)
    }

    private func stat(_ label: String, _ value: String, _ note: String) -> some View {
        VStack(spacing: 1) {
            Text(value)
                .font(.system(size: 15, weight: .semibold, design: .rounded))
                .monospacedDigit()
            Text(label)
                .font(.caption2)
                .foregroundStyle(.secondary)
            Text(note)
                .font(.system(size: 9))
                .foregroundStyle(.tertiary)
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity)
    }

    private func findings(_ report: Report) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Text("FINDINGS")
                    .font(.system(size: 10, weight: .semibold))
                    .foregroundStyle(.secondary)
                Spacer()
                SeverityTally(counts: report.summary.findings.bySeverity)
            }
            .padding(.horizontal, 14)
            .padding(.top, 10)
            .padding(.bottom, 6)

            // Only the worst few belong in a popover. The panel has the rest.
            ForEach(report.groupedFindings.prefix(4)) { group in
                Button {
                    openWindow(id: PanelWindow.id)
                    NSApp.activate(ignoringOtherApps: true)
                } label: {
                    FindingGroupRow(group: group, compact: true)
                }
                .buttonStyle(.plain)
            }

            if report.groupedFindings.count > 4 {
                Text("and \(report.groupedFindings.count - 4) more")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 14)
                    .padding(.bottom, 8)
            }
        }
    }

    private func reclaimRow(_ report: Report) -> some View {
        Button {
            openWindow(id: PanelWindow.id)
            NSApp.activate(ignoringOtherApps: true)
        } label: {
            HStack(spacing: 8) {
                Image(systemName: "trash")
                    .foregroundStyle(.secondary)
                VStack(alignment: .leading, spacing: 1) {
                    Text("\(Format.bytes(report.summary.images.reclaimableSize)) can be reclaimed")
                        .font(.callout)
                    Text("\(report.summary.images.unused) unused images · \(report.summary.volumes.unused) unused volumes")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Image(systemName: "chevron.right")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
            .padding(.horizontal, 14)
            .padding(.vertical, 9)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    // MARK: - Footer

    private var footer: some View {
        HStack(spacing: 12) {
            Button("Open Panel") {
                openWindow(id: PanelWindow.id)
                NSApp.activate(ignoringOtherApps: true)
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.small)

            if let updated = store.lastUpdated {
                Text(Format.relative(updated))
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }

            Spacer()

            Menu {
                Button("Share Docker health…") {
                    openWindow(id: PanelWindow.shareID)
                    NSApp.activate(ignoringOtherApps: true)
                }
                Button("Settings…") { openSettings() }
                Divider()
                if let version = store.binaryVersion {
                    Text("doctordock \(version)")
                }
                Button("Quit DoctorDock") { NSApp.terminate(nil) }
                    .keyboardShortcut("q")
            } label: {
                Image(systemName: "ellipsis.circle")
            }
            .menuStyle(.borderlessButton)
            .fixedSize()
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 9)
    }
}
