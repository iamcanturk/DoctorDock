import SwiftUI

/// Cleanup, with the same guards the CLI has.
///
/// The app cannot do anything the CLI would refuse: it shells out to the same
/// command, so the "--all never includes volumes" rule and the refusal to force
/// a removal are enforced in one place. What the app adds is seeing the whole
/// plan before agreeing to it. See docs/adr/0006-cleanup-safety-model.md.
struct CleanupView: View {
    @EnvironmentObject private var store: ScanStore

    @State private var targets = CleanupTargets.safeDefaults
    @State private var keepSince = KeepSince.none
    @State private var plan: CleanupPlan?
    @State private var isPlanning = false
    @State private var isApplying = false
    @State private var error: String?
    @State private var confirming = false
    @State private var typedConfirmation = ""
    @State private var result: CleanupPlan?

    enum KeepSince: String, CaseIterable, Identifiable {
        case none = "Everything"
        case day = "Older than 24 hours"
        case week = "Older than 7 days"
        case month = "Older than 30 days"

        var id: String { rawValue }

        var flagValue: String? {
            switch self {
            case .none: return nil
            case .day: return "24h"
            case .week: return "168h"
            case .month: return "720h"
            }
        }
    }

    var body: some View {
        VStack(spacing: 0) {
            controls
            Divider()
            body_
        }
        .task(id: taskKey) { await preview() }
        .alert("Remove these resources?", isPresented: $confirming) {
            confirmationButtons
        } message: {
            Text(confirmationMessage)
        }
    }

    private var taskKey: String {
        "\(targets.containers)\(targets.images)\(targets.networks)\(targets.volumes)\(keepSince.rawValue)"
    }

    // MARK: - Controls

    private var controls: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 16) {
                Toggle("Stopped containers", isOn: $targets.containers)
                Toggle("Unused images", isOn: $targets.images)
                Toggle("Unused networks", isOn: $targets.networks)
                Toggle("Unused volumes", isOn: $targets.volumes)
                    .toggleStyle(.checkbox)
                    .tint(.red)
                Spacer()
            }
            .toggleStyle(.checkbox)

            HStack(spacing: 12) {
                Picker("Remove:", selection: $keepSince) {
                    ForEach(KeepSince.allCases) { Text($0.rawValue).tag($0) }
                }
                .fixedSize()

                Button("Select everything except volumes") {
                    targets = .everythingRecreatable
                }
                .controlSize(.small)

                Spacer()

                if isPlanning { ProgressView().controlSize(.small) }
            }

            if targets.volumes {
                Label("Volumes hold data. Removing one cannot be undone, and an abandoned volume looks exactly like the only copy of a database.",
                      systemImage: "exclamationmark.triangle.fill")
                    .font(.caption)
                    .foregroundStyle(.red)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(14)
    }

    // MARK: - Body

    @ViewBuilder
    private var body_: some View {
        if let result {
            resultView(result)
        } else if let error {
            ContentUnavailableView("Could not plan the cleanup", systemImage: "exclamationmark.triangle",
                                   description: Text(error))
        } else if targets.isEmpty {
            ContentUnavailableView("Nothing selected", systemImage: "trash",
                                   description: Text("Choose what to consider above. Nothing is removed until you confirm."))
        } else if let plan, plan.items.isEmpty {
            ContentUnavailableView("Nothing to clean up", systemImage: "checkmark.seal",
                                   description: Text("No resources match what you selected."))
        } else if let plan {
            planView(plan)
        } else {
            ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
        }
    }

    private func planView(_ plan: CleanupPlan) -> some View {
        VStack(spacing: 0) {
            List {
                ForEach(plan.itemsByResource, id: \.kind) { group in
                    Section {
                        ForEach(group.items) { item in
                            CleanupRow(item: item)
                        }
                    } header: {
                        HStack {
                            Label(group.kind.plural, systemImage: group.kind.symbol)
                            Spacer()
                            let bytes = group.items.filter(\.hasKnownSize).reduce(Int64(0)) { $0 + $1.size }
                            if bytes > 0 {
                                Text(Format.bytes(bytes)).foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
            .listStyle(.inset)

            Divider()
            footer(plan)
        }
    }

    private func footer(_ plan: CleanupPlan) -> some View {
        HStack(spacing: 14) {
            VStack(alignment: .leading, spacing: 2) {
                Text("\(plan.summary.total) items")
                    .font(.headline)
                if plan.summary.reclaimableBytes > 0 {
                    Text("\(Format.bytes(plan.summary.reclaimableBytes)) would be reclaimed")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                } else {
                    Text("the daemon does not report a size for these")
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                }
            }

            HStack(spacing: 6) {
                ForEach(Risk.allCases, id: \.self) { risk in
                    let n = plan.summary.count(risk)
                    if n > 0 {
                        Label("\(n)", systemImage: risk.symbol)
                            .font(.caption)
                            .foregroundStyle(risk.color)
                            .help(risk.label)
                    }
                }
            }

            Spacer()

            if isApplying {
                ProgressView().controlSize(.small)
            }

            Button(plan.hasDataLoss ? "Remove — including volumes" : "Remove \(plan.summary.total) items") {
                typedConfirmation = ""
                confirming = true
            }
            .buttonStyle(.borderedProminent)
            .tint(plan.hasDataLoss ? .red : .accentColor)
            .disabled(isApplying || plan.items.isEmpty)
        }
        .padding(14)
    }

    private func resultView(_ result: CleanupPlan) -> some View {
        VStack(spacing: 14) {
            Image(systemName: result.summary.failed > 0 ? "exclamationmark.triangle.fill" : "checkmark.circle.fill")
                .font(.system(size: 40))
                .foregroundStyle(result.summary.failed > 0 ? .orange : .green)

            Text("\(result.summary.removed) removed")
                .font(.title2.weight(.semibold))

            if result.summary.reclaimedBytes > 0 {
                Text("\(Format.bytes(result.summary.reclaimedBytes)) reclaimed")
                    .foregroundStyle(.secondary)
            }

            if result.summary.failed > 0 {
                // A refusal is usually the daemon protecting something that
                // started being used since the scan, which is the system
                // working rather than failing.
                VStack(alignment: .leading, spacing: 4) {
                    Text("\(result.summary.failed) could not be removed")
                        .font(.callout.weight(.medium))
                    ForEach(result.items.filter { $0.error != nil }.prefix(5)) { item in
                        Text("\(item.name): \(item.error ?? "")")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                    }
                }
                .padding(12)
                .background(Color.secondary.opacity(0.08), in: RoundedRectangle(cornerRadius: 8))
            }

            Button("Done") {
                self.result = nil
                Task {
                    await store.refresh()
                    await preview()
                }
            }
            .buttonStyle(.borderedProminent)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(30)
    }

    // MARK: - Confirmation

    @ViewBuilder
    private var confirmationButtons: some View {
        if plan?.hasDataLoss == true {
            // Typing the word is the point: muscle memory for pressing Return
            // on a dialog is exactly what destroys a volume.
            TextField("Type delete", text: $typedConfirmation)
            Button("Delete", role: .destructive) { Task { await apply() } }
                .disabled(typedConfirmation.lowercased() != "delete")
            Button("Cancel", role: .cancel) {}
        } else {
            Button("Remove", role: .destructive) { Task { await apply() } }
            Button("Cancel", role: .cancel) {}
        }
    }

    private var confirmationMessage: String {
        guard let plan else { return "" }
        var lines = ["\(plan.summary.total) resources will be removed."]
        if plan.summary.reclaimableBytes > 0 {
            lines.append("This frees \(Format.bytes(plan.summary.reclaimableBytes)).")
        }
        if plan.hasDataLoss {
            let n = plan.summary.count(.dataLoss)
            lines.append("\(n) of them are volumes. Their contents cannot be recovered. Type \"delete\" to confirm.")
        }
        return lines.joined(separator: "\n\n")
    }

    // MARK: - Actions

    private func preview() async {
        guard !targets.isEmpty else {
            plan = nil
            return
        }
        isPlanning = true
        error = nil
        defer { isPlanning = false }

        do {
            plan = try await DoctorDockCLI.cleanupPreview(targets, keepSince: keepSince.flagValue)
        } catch {
            self.error = (error as? DoctorDockCLI.Failure)?.errorDescription ?? error.localizedDescription
            plan = nil
        }
    }

    private func apply() async {
        isApplying = true
        defer { isApplying = false }

        do {
            result = try await DoctorDockCLI.cleanupApply(targets, keepSince: keepSince.flagValue)
            plan = nil
        } catch {
            self.error = (error as? DoctorDockCLI.Failure)?.errorDescription ?? error.localizedDescription
        }
    }
}

private struct CleanupRow: View {
    let item: CleanupItem

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: item.risk.symbol)
                .foregroundStyle(item.risk.color)
                .font(.caption)

            VStack(alignment: .leading, spacing: 1) {
                Text(item.name)
                    .font(.callout)
                    .lineLimit(1)
                    .help(item.name)
                Text(item.reason)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }

            Spacer()

            if item.hasKnownSize {
                Text(Format.bytes(item.size))
                    .font(.caption)
                    .monospacedDigit()
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 2)
    }
}
