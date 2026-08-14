import SwiftUI

/// The right pane: what this finding is, why it matters, and how to fix it —
/// rendered natively from the structured explanation, not from terminal text.
struct FindingDetailView: View {
    let group: FindingGroup
    /// For previews/rendering: skip the async fetch and show this immediately.
    var preloaded: RuleExplanation?

    @State private var explanation: RuleExplanation?
    @State private var loading = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 22) {
                header
                affected

                if let e = explanation {
                    if !e.explanation.what.isEmpty {
                        DetailSection("What it looks for", e.explanation.what)
                    }
                    if !e.explanation.why.isEmpty {
                        DetailSection("Why it matters", e.explanation.why)
                    }
                    if let scenario = e.explanation.scenario, !scenario.isEmpty {
                        DetailSection("What goes wrong", scenario, tint: group.severity.color)
                    }
                    if !e.explanation.fixes.isEmpty {
                        fixes(e.explanation.fixes)
                    }
                    if let fp = e.explanation.falsePositives, !fp.isEmpty {
                        DetailSection("When this is fine to ignore", fp)
                    }
                    if let refs = e.explanation.references, !refs.isEmpty {
                        references(refs)
                    }
                } else {
                    // Until the explanation loads, the finding's own one-liners
                    // are shown, so the pane is never empty.
                    DetailSection("What is wrong", group.findings[0].description)
                    DetailSection("What to do", group.findings[0].recommendation)
                    if loading {
                        HStack(spacing: 6) {
                            ProgressView().controlSize(.small)
                            Text("Loading the full explanation…")
                                .font(.caption).foregroundStyle(.secondary)
                        }
                    }
                }
            }
            .padding(24)
            .frame(maxWidth: 640, alignment: .leading)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .task(id: group.id) {
            if let preloaded { explanation = preloaded; return }
            explanation = nil
            loading = true
            explanation = try? await DoctorDockCLI.explanation(group.ruleID)
            loading = false
        }
        .onAppear { if let preloaded { explanation = preloaded } }
    }

    // MARK: - Header

    private var header: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 8) {
                Label(group.severity.rawValue, systemImage: group.severity.symbol)
                    .font(.system(size: 11, weight: .bold))
                    .foregroundStyle(group.severity.color)
                    .padding(.horizontal, 8).padding(.vertical, 3)
                    .background(group.severity.color.opacity(0.15), in: Capsule())
                Text(group.ruleID)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(.secondary)
                Text(group.category.rawValue.lowercased())
                    .font(.caption)
                    .foregroundStyle(.tertiary)
                Spacer()
            }
            Text(group.rule)
                .font(.system(size: 24, weight: .semibold))
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var affected: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("AFFECTED · \(group.count)")
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(.secondary)
            FlowLayout(spacing: 6) {
                ForEach(group.findings) { finding in
                    HStack(spacing: 5) {
                        Image(systemName: group.resource.symbol)
                            .font(.system(size: 9))
                            .foregroundStyle(.tertiary)
                        Text(finding.resourceName)
                            .font(.system(size: 12, design: .monospaced))
                            .textSelection(.enabled)
                    }
                    .padding(.horizontal, 8).padding(.vertical, 4)
                    .background(Color.secondary.opacity(0.08), in: RoundedRectangle(cornerRadius: 6))
                }
            }
        }
    }

    private func fixes(_ fixes: [RuleExplanation.Fix]) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("HOW TO FIX IT")
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(.secondary)
            ForEach(Array(fixes.enumerated()), id: \.offset) { index, fix in
                VStack(alignment: .leading, spacing: 8) {
                    HStack(alignment: .top, spacing: 8) {
                        Text("\(index + 1)")
                            .font(.system(size: 12, weight: .bold))
                            .foregroundStyle(.white)
                            .frame(width: 20, height: 20)
                            .background(Color.accentColor, in: Circle())
                        Text(fix.title)
                            .font(.system(size: 14, weight: .medium))
                            .fixedSize(horizontal: false, vertical: true)
                    }
                    CodeBlock(code: fix.code, lang: fix.lang)
                        .padding(.leading, 28)
                }
            }
        }
    }

    private func references(_ refs: [RuleExplanation.Reference]) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("FURTHER READING")
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(.secondary)
            ForEach(refs) { ref in
                Link(destination: URL(string: ref.url) ?? URL(string: "https://docs.docker.com")!) {
                    HStack(spacing: 6) {
                        Image(systemName: "arrow.up.right.square")
                            .font(.system(size: 11))
                        Text(ref.title).font(.system(size: 13))
                    }
                }
                .buttonStyle(.plain)
                .foregroundStyle(Color.accentColor)
            }
        }
    }
}

/// A titled block of prose. Some explanations embed a small indented code
/// snippet mid-text; those lines are shown monospaced.
struct DetailSection: View {
    let title: String
    let body_: String
    var tint: Color?

    init(_ title: String, _ body: String, tint: Color? = nil) {
        self.title = title
        self.body_ = body
        self.tint = tint
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            Text(title.uppercased())
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(tint ?? .secondary)
            Text(body_)
                .font(.system(size: 14))
                .lineSpacing(3)
                .fixedSize(horizontal: false, vertical: true)
                .textSelection(.enabled)
                .padding(.leading, tint != nil ? 12 : 0)
                .overlay(alignment: .leading) {
                    if tint != nil {
                        RoundedRectangle(cornerRadius: 2)
                            .fill(tint!.opacity(0.5)).frame(width: 3)
                    }
                }
        }
    }
}

/// A copyable code snippet, styled like a terminal block.
struct CodeBlock: View {
    let code: String
    var lang: String = ""

    @State private var copied = false

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            if !lang.isEmpty {
                HStack {
                    Text(lang)
                        .font(.system(size: 10, weight: .medium, design: .monospaced))
                        .foregroundStyle(.secondary)
                    Spacer()
                    Button {
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setString(code, forType: .string)
                        copied = true
                        DispatchQueue.main.asyncAfter(deadline: .now() + 1.4) { copied = false }
                    } label: {
                        Label(copied ? "Copied" : "Copy",
                              systemImage: copied ? "checkmark" : "doc.on.doc")
                            .font(.system(size: 10, weight: .medium))
                    }
                    .buttonStyle(.plain)
                    .foregroundStyle(.secondary)
                }
                .padding(.horizontal, 12).padding(.vertical, 6)
                .background(Color.black.opacity(0.2))
            }
            Text(code)
                .font(.system(size: 12.5, design: .monospaced))
                .foregroundStyle(Color(red: 0.85, green: 0.87, blue: 0.9))
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(12)
        }
        .background(Color(red: 0.10, green: 0.11, blue: 0.13),
                    in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(.white.opacity(0.06)))
    }
}

/// A wrapping horizontal layout for the affected-resource chips.
struct FlowLayout: Layout {
    var spacing: CGFloat = 6

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let maxWidth = proposal.width ?? 600
        var rows: [[CGSize]] = [[]]
        var x: CGFloat = 0
        for view in subviews {
            let size = view.sizeThatFits(.unspecified)
            if x + size.width > maxWidth, !rows[rows.count - 1].isEmpty {
                rows.append([]); x = 0
            }
            rows[rows.count - 1].append(size)
            x += size.width + spacing
        }
        let height = rows.reduce(CGFloat(0)) { acc, row in
            acc + (row.map(\.height).max() ?? 0) + spacing
        }
        return CGSize(width: maxWidth, height: max(0, height - spacing))
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize,
                       subviews: Subviews, cache: inout ()) {
        var x = bounds.minX
        var y = bounds.minY
        var rowHeight: CGFloat = 0
        for view in subviews {
            let size = view.sizeThatFits(.unspecified)
            if x + size.width > bounds.maxX, x > bounds.minX {
                x = bounds.minX
                y += rowHeight + spacing
                rowHeight = 0
            }
            view.place(at: CGPoint(x: x, y: y), proposal: ProposedViewSize(size))
            x += size.width + spacing
            rowHeight = max(rowHeight, size.height)
        }
    }
}
