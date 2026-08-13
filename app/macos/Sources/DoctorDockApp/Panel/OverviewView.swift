import SwiftUI

struct OverviewView: View {
    let report: Report

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                header

                grid

                if !report.findings.isEmpty {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("WHAT TO FIX FIRST")
                            .font(.system(size: 10, weight: .semibold))
                            .foregroundStyle(.secondary)

                        VStack(spacing: 0) {
                            ForEach(report.groupedFindings.prefix(5)) { group in
                                FindingGroupRow(group: group)
                                if group.id != report.groupedFindings.prefix(5).last?.id {
                                    Divider().padding(.leading, 38)
                                }
                            }
                        }
                        .background(Color.secondary.opacity(0.06),
                                    in: RoundedRectangle(cornerRadius: 8))
                    }
                }
            }
            .padding(20)
        }
    }

    private var header: some View {
        HStack(alignment: .center, spacing: 16) {
            ScoreBadge(score: report.score, size: 72)

            VStack(alignment: .leading, spacing: 3) {
                Text(Format.grade(report.score).capitalized)
                    .font(.title2.weight(.semibold))
                Text(report.docker.display)
                    .font(.callout)
                    .foregroundStyle(.secondary)
                Text("100 means no findings. The score is for comparing this machine to itself over time.")
                    .font(.caption)
                    .foregroundStyle(.tertiary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer()
        }
    }

    private var grid: some View {
        LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 10), count: 4), spacing: 10) {
            StatTile(title: "Containers",
                     value: "\(report.summary.containers.total)",
                     note: "\(report.summary.containers.running) running · \(report.summary.containers.stopped) stopped")
            StatTile(title: "Images",
                     value: "\(report.summary.images.total)",
                     note: Format.bytes(report.summary.images.totalSize))
            StatTile(title: "Volumes",
                     value: "\(report.summary.volumes.total)",
                     note: "\(report.summary.volumes.anonymous) unnamed")
            StatTile(title: "Networks",
                     value: "\(report.summary.networks.custom)",
                     note: "you created these")

            StatTile(title: "Unhealthy",
                     value: "\(report.summary.containers.unhealthy)",
                     note: "failing their healthcheck",
                     tint: report.summary.containers.unhealthy > 0 ? .orange : nil)
            StatTile(title: "Unused images",
                     value: "\(report.summary.images.unused)",
                     note: "\(Format.bytes(report.summary.images.reclaimableSize)) reclaimable")
            StatTile(title: "Unused volumes",
                     value: "\(report.summary.volumes.unused)",
                     note: "nothing mounts them")
            StatTile(title: "Unused networks",
                     value: "\(report.summary.networks.unused)",
                     note: "nothing attached")
        }
    }
}
