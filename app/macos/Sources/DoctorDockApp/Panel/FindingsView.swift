import SwiftUI

/// Findings, grouped by rule, with a natively-rendered explanation on the side.
///
/// The left rail is a custom scrolling list rather than a `List`: it gives full
/// control over the selected-row treatment and renders predictably. The right
/// pane renders the structured explanation from `doctordock explain --format
/// json` — sections, code blocks and links — instead of dumping terminal text.
struct FindingsView: View {
    let report: Report

    @State private var selectedID: String?
    @State private var severityFilter: Severity?
    @State private var search = ""

    private var groups: [FindingGroup] { report.groupedFindings }

    private var filtered: [FindingGroup] {
        groups.filter { group in
            if let severityFilter, group.severity != severityFilter { return false }
            guard !search.isEmpty else { return true }
            let needle = search.lowercased()
            return group.rule.lowercased().contains(needle)
                || group.ruleID.lowercased().contains(needle)
                || group.resourceNames.contains { $0.lowercased().contains(needle) }
        }
    }

    private var selected: FindingGroup? {
        filtered.first { $0.id == selectedID } ?? filtered.first
    }

    var body: some View {
        HStack(spacing: 0) {
            rail
                .frame(width: 340)
            Divider()
            detail
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .onAppear { if selectedID == nil { selectedID = filtered.first?.id } }
    }

    // MARK: - Left rail

    private var rail: some View {
        VStack(spacing: 0) {
            filterBar
            Divider()
            if filtered.isEmpty {
                Spacer()
                Text("Nothing matches")
                    .foregroundStyle(.secondary)
                Spacer()
            } else {
                ScrollView {
                    LazyVStack(spacing: 6) {
                        ForEach(filtered) { group in
                            FindingCard(group: group, selected: group.id == selected?.id)
                                .onTapGesture { selectedID = group.id }
                        }
                    }
                    .padding(10)
                }
            }
        }
        .background(.background.opacity(0.4))
    }

    private var filterBar: some View {
        VStack(spacing: 8) {
            SeverityFilterBar(counts: report.summary.findings.bySeverity,
                              selection: $severityFilter)
            HStack(spacing: 6) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.tertiary)
                    .font(.caption)
                TextField("Filter findings", text: $search)
                    .textFieldStyle(.plain)
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 6)
            .background(Color.secondary.opacity(0.1), in: RoundedRectangle(cornerRadius: 7))
        }
        .padding(12)
    }

    // MARK: - Detail

    @ViewBuilder
    private var detail: some View {
        if let selected {
            FindingDetailView(group: selected)
                .id(selected.id)
        } else {
            ContentUnavailableView("No findings",
                systemImage: "checkmark.seal",
                description: Text("This environment looks healthy."))
        }
    }
}

/// The per-severity filter, as a row of counted pills. Tapping one filters;
/// tapping it again clears.
struct SeverityFilterBar: View {
    let counts: SeverityCounts
    @Binding var selection: Severity?

    // Short codes keep every chip on one line inside the 340pt rail; the full
    // name is on the tooltip and everywhere else in the UI.
    private func shortLabel(_ severity: Severity) -> String {
        switch severity {
        case .critical: return "Crit"
        case .high: return "High"
        case .medium: return "Med"
        case .low: return "Low"
        case .info: return "Info"
        }
    }

    var body: some View {
        HStack(spacing: 5) {
            chip(label: "All", count: counts.total, color: .secondary,
                 active: selection == nil, help: "All findings") { selection = nil }

            ForEach(Severity.allCases.reversed(), id: \.self) { severity in
                let n = counts.count(severity)
                if n > 0 {
                    chip(label: shortLabel(severity), count: n,
                         color: severity.color, active: selection == severity,
                         help: severity.rawValue.capitalized) {
                        selection = selection == severity ? nil : severity
                    }
                }
            }
            Spacer(minLength: 0)
        }
    }

    private func chip(label: String, count: Int, color: Color,
                      active: Bool, help: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 4) {
                Text(label)
                    .font(.system(size: 11, weight: .medium))
                    .lineLimit(1)
                Text("\(count)")
                    .font(.system(size: 11, weight: .semibold))
                    .monospacedDigit()
                    .opacity(0.75)
            }
            .fixedSize()
            .padding(.horizontal, 7)
            .padding(.vertical, 4)
            .background(active ? color.opacity(0.22) : Color.secondary.opacity(0.08),
                        in: Capsule())
            .foregroundStyle(active ? color : .secondary)
            .overlay(Capsule().strokeBorder(active ? color.opacity(0.45) : .clear))
        }
        .buttonStyle(.plain)
        .help(help)
    }
}

/// One finding group in the left rail.
struct FindingCard: View {
    let group: FindingGroup
    let selected: Bool

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            RoundedRectangle(cornerRadius: 2)
                .fill(group.severity.color)
                .frame(width: 3)

            VStack(alignment: .leading, spacing: 3) {
                HStack(spacing: 6) {
                    Image(systemName: group.severity.symbol)
                        .foregroundStyle(group.severity.color)
                        .font(.system(size: 11))
                    Text(group.rule)
                        .font(.system(size: 13, weight: .medium))
                        .foregroundStyle(.primary)
                        .lineLimit(1)
                    Spacer(minLength: 0)
                    if group.count > 1 {
                        Text("\(group.count)")
                            .font(.system(size: 10, weight: .semibold))
                            .monospacedDigit()
                            .padding(.horizontal, 5).padding(.vertical, 1)
                            .background(Color.secondary.opacity(0.15), in: Capsule())
                            .foregroundStyle(.secondary)
                    }
                }
                Text(subtitle)
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
        }
        .padding(.vertical, 8)
        .padding(.horizontal, 9)
        .background(selected ? Color.accentColor.opacity(0.15) : .clear,
                    in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8)
            .strokeBorder(selected ? Color.accentColor.opacity(0.4) : .clear))
        .contentShape(Rectangle())
    }

    private var subtitle: String {
        if group.count == 1 { return group.findings[0].resourceName }
        let shown = group.resourceNames.prefix(2).joined(separator: ", ")
        return group.count > 2 ? "\(shown) +\(group.count - 2)" : shown
    }
}
