import SwiftUI

/// The full window: a sidebar of sections over one shared report.
struct PanelView: View {
    @EnvironmentObject private var store: ScanStore
    @State private var section: Section = .overview

    enum Section: String, CaseIterable, Identifiable {
        case overview = "Overview"
        case findings = "Findings"
        case containers = "Containers"
        case images = "Images"
        case volumes = "Volumes"
        case networks = "Networks"
        case cleanup = "Cleanup"

        var id: String { rawValue }

        var symbol: String {
            switch self {
            case .overview: return "square.grid.2x2"
            case .findings: return "exclamationmark.triangle"
            case .containers: return "shippingbox"
            case .images: return "square.stack.3d.up"
            case .volumes: return "externaldrive"
            case .networks: return "network"
            case .cleanup: return "trash"
            }
        }
    }

    var body: some View {
        NavigationSplitView {
            List(Section.allCases, selection: $section) { item in
                Label(item.rawValue, systemImage: item.symbol)
                    .badge(badge(for: item))
                    .tag(item)
            }
            .navigationSplitViewColumnWidth(min: 170, ideal: 190, max: 240)
        } detail: {
            Group {
                switch store.state {
                case .idle, .scanning where store.report == nil:
                    ProgressView("Scanning…")
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                case .failed(let failure) where store.report == nil:
                    FailureView(failure: failure) {
                        Task { await store.refresh() }
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .padding(40)
                default:
                    if let report = store.report {
                        detail(for: report)
                    }
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .toolbar {
            ToolbarItem(placement: .status) {
                if let updated = store.lastUpdated {
                    Text("Updated \(Format.relative(updated))")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            ToolbarItem(placement: .primaryAction) {
                Button {
                    Task { await store.refresh() }
                } label: {
                    Image(systemName: "arrow.clockwise")
                }
                .disabled(store.isScanning)
                .help("Rescan (⌘R)")
            }
        }
    }

    @ViewBuilder
    private func detail(for report: Report) -> some View {
        switch section {
        case .overview:   OverviewView(report: report)
        case .findings:   FindingsView(report: report)
        case .containers: ContainersView(report: report)
        case .images:     ImagesView(report: report)
        case .volumes:    VolumesView(report: report)
        case .networks:   NetworksView(report: report)
        case .cleanup:    CleanupView()
        }
    }

    /// Counts in the sidebar, so the shape of the environment is visible
    /// without clicking through every section.
    private func badge(for item: Section) -> Int {
        guard let report = store.report else { return 0 }
        switch item {
        case .overview: return 0
        case .findings: return report.summary.findings.total
        case .containers: return report.summary.containers.total
        case .images: return report.summary.images.total
        case .volumes: return report.summary.volumes.total
        case .networks: return report.summary.networks.total
        case .cleanup: return report.summary.images.unused + report.summary.volumes.unused
            + report.summary.networks.unused
        }
    }
}
