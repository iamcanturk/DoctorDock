import Foundation

// Swift mirror of the `doctordock cleanup` document.
//
// The safety model is enforced by the core, not here: the app can only ask for
// what the CLI already refuses to do carelessly. See
// docs/adr/0006-cleanup-safety-model.md.

enum Risk: String, Codable, CaseIterable {
    /// Docker's own prune would remove it and it cannot be needed again.
    case safe
    /// Removable, but the user may have wanted it.
    case review
    /// May destroy the only copy of real data. Volumes only.
    case dataLoss = "data-loss"
    case unknown

    init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = Risk(rawValue: raw) ?? .unknown
    }

    var label: String {
        switch self {
        case .safe: return "Safe"
        case .review: return "Worth reviewing"
        case .dataLoss: return "Could destroy data"
        case .unknown: return "Unknown"
        }
    }
}

struct CleanupItem: Codable, Identifiable, Hashable {
    let resource: ResourceKind
    let id_: String
    let name: String
    let reason: String
    let risk: Risk
    /// Bytes reclaimed, or -1 when the daemon reports no size.
    let size: Int64
    let removed: Bool
    let error: String?

    enum CodingKeys: String, CodingKey {
        case resource
        case id_ = "id"
        case name, reason, risk, size, removed, error
    }

    var id: String { "\(resource.rawValue)|\(id_)" }
    var hasKnownSize: Bool { size > 0 }
}

struct CleanupSummary: Codable, Hashable {
    let total: Int
    let byResource: [String: Int]
    let byRisk: [String: Int]
    let reclaimableBytes: Int64
    let removed: Int
    let failed: Int
    let reclaimedBytes: Int64

    func count(_ risk: Risk) -> Int { byRisk[risk.rawValue] ?? 0 }
}

struct CleanupPlan: Codable {
    let schemaVersion: String
    let generatedAt: Date
    let tool: ToolInfo
    /// False for a dry run. The UI must check this before telling anyone
    /// something was deleted.
    let applied: Bool
    let items: [CleanupItem]
    let summary: CleanupSummary

    var itemsByResource: [(kind: ResourceKind, items: [CleanupItem])] {
        // Volumes last, matching the order the core removes them in, so the
        // irreversible group is the one the eye lands on after everything else.
        let order: [ResourceKind] = [.container, .image, .network, .volume]
        return order.compactMap { kind in
            let matching = items.filter { $0.resource == kind }
            return matching.isEmpty ? nil : (kind, matching)
        }
    }

    var hasDataLoss: Bool { items.contains { $0.risk == .dataLoss } }
}

/// What the user asked to clean. Mirrors the CLI's target flags, including the
/// rule that `all` never covers volumes.
struct CleanupTargets: Equatable {
    var containers = false
    var images = false
    var networks = false
    var volumes = false

    /// Everything that can be recreated. Volumes are absent by design.
    static let everythingRecreatable = CleanupTargets(
        containers: true, images: true, networks: true, volumes: false
    )

    /// What the CLI considers with no flags at all.
    static let safeDefaults = CleanupTargets(
        containers: false, images: false, networks: true, volumes: false
    )

    var isEmpty: Bool { !containers && !images && !networks && !volumes }

    var flags: [String] {
        var out: [String] = []
        if containers { out.append("--containers") }
        if images { out.append("--images") }
        if networks { out.append("--networks") }
        if volumes { out.append("--volumes") }
        return out
    }
}
