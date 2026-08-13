import SwiftUI

/// The four resource tables.
///
/// Each shows an issue count per row, taken from the same findings the
/// Findings section lists — so a container with three problems reads the same
/// wherever you look at it.

private func issueCounts(_ report: Report, _ kind: ResourceKind) -> [String: (count: Int, worst: Severity)] {
    var out: [String: (Int, Severity)] = [:]
    for finding in report.findings where finding.resource == kind {
        let existing = out[finding.resourceId]
        let worst = max(existing?.1 ?? .info, finding.severity)
        out[finding.resourceId] = ((existing?.0 ?? 0) + 1, worst)
    }
    return out
}

/// The issue-count cell, quiet when a row is clean.
private struct IssueBadge: View {
    let entry: (count: Int, worst: Severity)?

    var body: some View {
        if let entry {
            Text("\(entry.count)")
                .font(.system(size: 10, weight: .semibold))
                .monospacedDigit()
                .padding(.horizontal, 5)
                .padding(.vertical, 1)
                .background(entry.worst.color.opacity(0.16), in: Capsule())
                .foregroundStyle(entry.worst.color)
        } else {
            Text("—").foregroundStyle(.quaternary)
        }
    }
}

private struct SearchBar: View {
    @Binding var text: String
    var placeholder: String

    var body: some View {
        TextField(placeholder, text: $text)
            .textFieldStyle(.roundedBorder)
            .padding(10)
    }
}

// MARK: - Containers

struct ContainersView: View {
    let report: Report
    @State private var search = ""

    private var rows: [Container] {
        let all = report.containers ?? []
        guard !search.isEmpty else { return all }
        let needle = search.lowercased()
        return all.filter {
            $0.name.lowercased().contains(needle) || $0.image.lowercased().contains(needle)
        }
    }

    var body: some View {
        let issues = issueCounts(report, .container)

        VStack(spacing: 0) {
            SearchBar(text: $search, placeholder: "Filter containers")
            Divider()

            Table(rows) {
                TableColumn("Name") { Text($0.name).fontWeight(.medium) }.width(min: 140)
                TableColumn("Image") { Text($0.image).foregroundStyle(.secondary) }.width(min: 140)
                TableColumn("State") { container in
                    Text(container.state)
                        .foregroundStyle(container.isRunning ? .green : .secondary)
                }
                .width(80)
                TableColumn("Health") { container in
                    // A stopped container keeps its last health status; showing
                    // that in red would read as a live failure.
                    Text(container.health)
                        .foregroundStyle(healthColor(container))
                }
                .width(80)
                TableColumn("Ports") { container in
                    Text(container.ports.isEmpty ? "—"
                         : container.ports.map(\.display).joined(separator: " "))
                        .font(.system(.caption, design: .monospaced))
                        .foregroundStyle(container.ports.contains(where: \.isPubliclyBound)
                                         ? .orange : .secondary)
                }
                .width(min: 120)
                TableColumn("User") { Text($0.effectiveUser).foregroundStyle(.secondary) }.width(80)
                TableColumn("Issues") { IssueBadge(entry: issues[$0.id]) }.width(60)
            }
        }
    }

    private func healthColor(_ container: Container) -> Color {
        guard container.isRunning else { return .secondary }
        switch container.health {
        case "healthy": return .green
        case "unhealthy": return .red
        case "starting": return .orange
        default: return .secondary
        }
    }
}

// MARK: - Images

struct ImagesView: View {
    let report: Report
    @State private var search = ""
    @State private var unusedOnly = false

    private var rows: [DockerImage] {
        var all = report.images ?? []
        if unusedOnly { all = all.filter { !$0.inUse } }
        guard !search.isEmpty else { return all }
        let needle = search.lowercased()
        return all.filter { $0.displayName.lowercased().contains(needle) }
    }

    var body: some View {
        let issues = issueCounts(report, .image)

        VStack(spacing: 0) {
            HStack {
                SearchBar(text: $search, placeholder: "Filter images")
                Toggle("Unused only", isOn: $unusedOnly)
                    .toggleStyle(.checkbox)
                    .padding(.trailing, 10)
            }
            Divider()

            Table(rows) {
                TableColumn("Repository:Tag") { image in
                    Text(image.displayName)
                        .fontWeight(image.dangling ? .regular : .medium)
                        .foregroundStyle(image.dangling ? .secondary : .primary)
                }
                .width(min: 200)
                TableColumn("Size") { Text(Format.bytes($0.size)) }.width(90)
                TableColumn("Created") { Text(Format.age($0.created)) }.width(100)
                TableColumn("Used by") { image in
                    Text(image.usedBy?.joined(separator: ", ") ?? "—")
                        .foregroundStyle(image.inUse ? .secondary : .quaternary)
                }
                .width(min: 140)
                TableColumn("Issues") { IssueBadge(entry: issues[$0.id]) }.width(60)
            }
        }
    }
}

// MARK: - Volumes

struct VolumesView: View {
    let report: Report
    @State private var search = ""

    private var rows: [Volume] {
        let all = report.volumes ?? []
        guard !search.isEmpty else { return all }
        return all.filter { $0.name.lowercased().contains(search.lowercased()) }
    }

    var body: some View {
        let issues = issueCounts(report, .volume)

        VStack(spacing: 0) {
            SearchBar(text: $search, placeholder: "Filter volumes")
            Divider()

            Table(rows) {
                TableColumn("Name") { volume in
                    Text(volume.isAnonymous ? String(volume.name.prefix(16)) + "…" : volume.name)
                        .font(volume.isAnonymous ? .system(.body, design: .monospaced) : .body)
                        .help(volume.name)
                }
                .width(min: 200)
                TableColumn("Kind") { volume in
                    Text(volume.isAnonymous ? "unnamed" : "named")
                        .foregroundStyle(.secondary)
                }
                .width(80)
                TableColumn("Used by") { volume in
                    Text(volume.usedBy?.joined(separator: ", ") ?? "—")
                        .foregroundStyle(volume.inUse ? .secondary : .quaternary)
                }
                .width(min: 140)
                TableColumn("Issues") { IssueBadge(entry: issues[$0.name]) }.width(60)
            }

            Divider()
            Label("DoctorDock never removes a volume unless you ask for it explicitly. Check the contents first — an abandoned volume and the only copy of a database look identical.",
                  systemImage: "info.circle")
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(10)
        }
    }
}

// MARK: - Networks

struct NetworksView: View {
    let report: Report

    var body: some View {
        let issues = issueCounts(report, .network)

        Table(report.networks ?? []) {
            TableColumn("Name") { network in
                Text(network.name)
                    .fontWeight(network.isBuiltin ? .regular : .medium)
                    .foregroundStyle(network.isBuiltin ? .secondary : .primary)
            }
            .width(min: 160)
            TableColumn("Driver") { Text($0.driver).foregroundStyle(.secondary) }.width(90)
            TableColumn("Subnet") { network in
                Text(network.subnets?.first ?? "—")
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)
            }
            .width(140)
            TableColumn("Containers") { network in
                Text(network.containers.isEmpty ? "—" : network.containers.joined(separator: ", "))
                    .foregroundStyle(network.containers.isEmpty ? .quaternary : .secondary)
            }
            .width(min: 160)
            TableColumn("Issues") { IssueBadge(entry: issues[$0.id]) }.width(60)
        }
    }
}
