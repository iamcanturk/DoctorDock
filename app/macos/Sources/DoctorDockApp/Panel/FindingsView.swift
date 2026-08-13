import SwiftUI

/// Every finding, grouped by rule, with the full explanation on demand.
struct FindingsView: View {
    let report: Report

    @State private var selected: FindingGroup?
    @State private var severityFilter: Severity?
    @State private var search = ""

    var body: some View {
        HSplitView {
            list
                .frame(minWidth: 320, idealWidth: 380)

            if let selected {
                FindingDetailView(group: selected)
                    .frame(minWidth: 360)
            } else {
                ContentUnavailableView(
                    "Select a finding",
                    systemImage: "sidebar.right",
                    description: Text("Pick one on the left to see what it means and how to fix it.")
                )
                .frame(minWidth: 360)
            }
        }
    }

    private var filtered: [FindingGroup] {
        report.groupedFindings.filter { group in
            if let severityFilter, group.severity != severityFilter { return false }
            guard !search.isEmpty else { return true }
            let needle = search.lowercased()
            return group.rule.lowercased().contains(needle)
                || group.ruleID.lowercased().contains(needle)
                || group.resourceNames.contains { $0.lowercased().contains(needle) }
        }
    }

    private var list: some View {
        VStack(spacing: 0) {
            HStack(spacing: 8) {
                Picker("", selection: $severityFilter) {
                    Text("All").tag(Severity?.none)
                    ForEach(Severity.allCases.reversed(), id: \.self) { severity in
                        if report.summary.findings.bySeverity.count(severity) > 0 {
                            Text(severity.rawValue.capitalized).tag(Severity?.some(severity))
                        }
                    }
                }
                .pickerStyle(.menu)
                .fixedSize()

                TextField("Filter", text: $search)
                    .textFieldStyle(.roundedBorder)
            }
            .padding(10)

            Divider()

            if filtered.isEmpty {
                ContentUnavailableView("Nothing matches", systemImage: "magnifyingglass")
                    .frame(maxHeight: .infinity)
            } else {
                List(filtered, selection: Binding(
                    get: { selected?.id },
                    set: { id in selected = filtered.first { $0.id == id } }
                )) { group in
                    FindingGroupRow(group: group)
                        .tag(group.id)
                }
                .listStyle(.inset)
            }
        }
    }
}

/// One rule's findings, plus the CLI's own explanation of it.
struct FindingDetailView: View {
    let group: FindingGroup

    @State private var explanation: String?
    @State private var loadingExplanation = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                header

                section("WHAT IS WRONG", group.findings[0].description)
                section("WHAT TO DO", group.findings[0].recommendation)

                affected

                explanationBlock
            }
            .padding(20)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .task(id: group.id) {
            explanation = nil
            await loadExplanation()
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 8) {
                Label(group.severity.rawValue, systemImage: group.severity.symbol)
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(group.severity.color)
                    .padding(.horizontal, 7)
                    .padding(.vertical, 2)
                    .background(group.severity.color.opacity(0.14), in: Capsule())

                Text(group.ruleID)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)

                Text(group.category.rawValue.lowercased())
                    .font(.caption)
                    .foregroundStyle(.tertiary)
            }

            Text(group.rule)
                .font(.title3.weight(.semibold))
        }
    }

    private func section(_ title: String, _ body: String) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title)
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(.secondary)
            Text(body)
                .font(.callout)
                .fixedSize(horizontal: false, vertical: true)
                .textSelection(.enabled)
        }
    }

    private var affected: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text("AFFECTED (\(group.count))")
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(.secondary)

            VStack(alignment: .leading, spacing: 0) {
                ForEach(group.findings) { finding in
                    HStack(spacing: 6) {
                        Image(systemName: group.resource.symbol)
                            .font(.caption2)
                            .foregroundStyle(.tertiary)
                        Text(finding.resourceName)
                            .font(.system(.callout, design: .monospaced))
                            .textSelection(.enabled)
                        Spacer()
                        if let detail = finding.details?.values.first, group.count == 1 {
                            Text(detail)
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                                .lineLimit(1)
                        }
                    }
                    .padding(.horizontal, 9)
                    .padding(.vertical, 4)
                }
            }
            .background(Color.secondary.opacity(0.06), in: RoundedRectangle(cornerRadius: 6))
        }
    }

    /// The long-form explanation comes from `doctordock explain`, not from a
    /// copy in the app. One source, so the two can never disagree.
    @ViewBuilder
    private var explanationBlock: some View {
        VStack(alignment: .leading, spacing: 5) {
            HStack {
                Text("FULL EXPLANATION")
                    .font(.system(size: 10, weight: .semibold))
                    .foregroundStyle(.secondary)
                if loadingExplanation {
                    ProgressView().controlSize(.small)
                }
            }

            if let explanation, !explanation.isEmpty {
                Text(explanation)
                    .font(.system(.caption, design: .monospaced))
                    .textSelection(.enabled)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(10)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color.secondary.opacity(0.06),
                                in: RoundedRectangle(cornerRadius: 6))
            } else if !loadingExplanation {
                Text("Run `doctordock explain \(group.ruleID)` in a terminal for the full write-up.")
                    .font(.caption)
                    .foregroundStyle(.tertiary)
            }
        }
    }

    private func loadExplanation() async {
        loadingExplanation = true
        defer { loadingExplanation = false }
        explanation = try? await DoctorDockCLI.explain(group.ruleID)
    }
}
