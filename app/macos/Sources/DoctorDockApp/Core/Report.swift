import Foundation

// Swift mirrors of the DoctorDock JSON contract.
//
// Field names come from `.convertFromSnakeCase`, so these track pkg/model
// exactly. The contract is documented in docs/JSON_SCHEMA.md and versioned by
// `schemaVersion`; see docs/adr/0004-json-as-the-gui-contract.md for why the
// app talks to the core over JSON rather than linking it.
//
// Every optional here is optional in the contract too. Decoding must not fail
// because a daemon did not report a kernel version.

enum Severity: String, Codable, CaseIterable, Comparable {
    case info = "INFO"
    case low = "LOW"
    case medium = "MEDIUM"
    case high = "HIGH"
    case critical = "CRITICAL"

    /// Ordering position, lowest first. Mirrors Severity.Rank in the core.
    var rank: Int {
        switch self {
        case .info: return 0
        case .low: return 1
        case .medium: return 2
        case .high: return 3
        case .critical: return 4
        }
    }

    static func < (lhs: Severity, rhs: Severity) -> Bool { lhs.rank < rhs.rank }

    /// An unknown severity from a newer core decodes as `.info` rather than
    /// failing the whole document — a minor schema bump may add a level.
    init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = Severity(rawValue: raw) ?? .info
    }
}

enum Category: String, Codable {
    case security = "SECURITY"
    case performance = "PERFORMANCE"
    case resource = "RESOURCE"
    case configuration = "CONFIGURATION"
    case cleanup = "CLEANUP"
    case unknown

    init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = Category(rawValue: raw) ?? .unknown
    }
}

enum ResourceKind: String, Codable {
    case container
    case image
    case volume
    case network
    case system
    case unknown

    init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = ResourceKind(rawValue: raw) ?? .unknown
    }
}

struct Finding: Codable, Identifiable, Hashable {
    let id_: String
    let rule: String
    let severity: Severity
    let category: Category
    let resource: ResourceKind
    let resourceId: String
    let resourceName: String
    let title: String
    let description: String
    let recommendation: String
    let details: [String: String]?

    enum CodingKeys: String, CodingKey {
        case id_ = "id"
        case rule, severity, category, resource
        case resourceId, resourceName, title, description, recommendation, details
    }

    /// Identifiable needs a value unique per row. A rule can fire more than
    /// once for one resource — DD004 reports each sensitive mount separately —
    /// so the rule ID alone is not enough.
    var id: String { "\(id_)|\(resourceId)|\(title)" }

    /// The rule ID, e.g. "DD005". Named separately from `id` because that is
    /// taken by Identifiable.
    var ruleID: String { id_ }
}

struct SeverityCounts: Codable, Hashable {
    let info: Int
    let low: Int
    let medium: Int
    let high: Int
    let critical: Int

    func count(_ severity: Severity) -> Int {
        switch severity {
        case .info: return info
        case .low: return low
        case .medium: return medium
        case .high: return high
        case .critical: return critical
        }
    }

    var total: Int { info + low + medium + high + critical }
}

struct ContainerSummary: Codable, Hashable {
    let total, running, stopped, paused, restarting, created, unhealthy: Int
}

struct ImageSummary: Codable, Hashable {
    let total, dangling, unused: Int
    let totalSize, reclaimableSize: Int64
}

struct VolumeSummary: Codable, Hashable {
    let total, unused, anonymous: Int
}

struct NetworkSummary: Codable, Hashable {
    let total, custom, unused: Int
}

struct FindingSummary: Codable, Hashable {
    let total: Int
    let bySeverity: SeverityCounts
    let byCategory: [String: Int]
}

struct Summary: Codable, Hashable {
    let containers: ContainerSummary
    let images: ImageSummary
    let volumes: VolumeSummary
    let networks: NetworkSummary
    let findings: FindingSummary
}

struct DockerInfo: Codable, Hashable {
    let serverVersion: String
    let apiVersion: String
    let osType: String
    let architecture: String
    let kernelVersion: String?
    let operatingSystem: String?
    let storageDriver: String?
    let cgroupVersion: String?
    let cpus: Int?
    let memTotal: Int64?
    let rootless: Bool
    let securityOptions: [String]?

    var display: String {
        [operatingSystem, "Docker \(serverVersion)", "\(osType)/\(architecture)"]
            .compactMap { $0 }
            .joined(separator: " · ")
    }
}

struct ToolInfo: Codable, Hashable {
    let name: String
    let version: String
    let commit: String?
}

struct Port: Codable, Hashable {
    let privatePort: Int
    let publicPort: Int?
    let type: String
    let hostIP: String?

    enum CodingKeys: String, CodingKey {
        case privatePort, publicPort, type
        case hostIP = "hostIp"
    }

    var isPublished: Bool { (publicPort ?? 0) != 0 }

    /// Reachable from off-host, which is what makes DD006 fire.
    var isPubliclyBound: Bool {
        guard isPublished else { return false }
        switch hostIP {
        case nil, "", "0.0.0.0", "::", "[::]": return true
        default: return false
        }
    }

    var display: String {
        guard let publicPort, isPublished else { return "\(privatePort)/\(type)" }
        return "\(publicPort)→\(privatePort)/\(type)"
    }
}

struct Mount: Codable, Hashable {
    let type: String
    let source: String
    let destination: String
    let name: String?
    let readOnly: Bool
}

struct Container: Codable, Identifiable, Hashable {
    let id: String
    let name: String
    let image: String
    let imageId: String
    let state: String
    let status: String
    let created: Date
    let startedAt: Date?
    let ports: [Port]
    let mounts: [Mount]
    let networks: [String]
    let restartPolicy: String
    let restartCount: Int
    let hasHealthcheck: Bool
    let health: String
    let user: String
    let effectiveUser: String
    let privileged: Bool
    let networkMode: String
    let pidMode: String
    let ipcMode: String
    let capAdd: [String]
    let capDrop: [String]
    let readOnlyRootfs: Bool
    let memoryLimit: Int64
    let nanoCpus: Int64
    let pidsLimit: Int64
    /// Variable names only — values never leave the daemon.
    /// See docs/adr/0005-no-secret-collection.md.
    let envKeys: [String]
    let labels: [String: String]?

    var isRunning: Bool { state == "running" }
}

struct DockerImage: Codable, Identifiable, Hashable {
    let id: String
    let repoTags: [String]
    let repoDigests: [String]?
    let size: Int64
    let sharedSize: Int64?
    let created: Date
    let architecture: String?
    let os: String?
    let layers: Int?
    let dangling: Bool
    let inUse: Bool
    let usedBy: [String]?
    let labels: [String: String]?

    var displayName: String {
        repoTags.first(where: { $0 != "<none>:<none>" }) ?? shortID
    }

    var shortID: String {
        let trimmed = id.hasPrefix("sha256:") ? String(id.dropFirst(7)) : id
        return String(trimmed.prefix(12))
    }
}

struct Volume: Codable, Identifiable, Hashable {
    let name: String
    let driver: String
    let mountpoint: String
    let scope: String?
    let created: Date?
    let size: Int64?
    let inUse: Bool
    let usedBy: [String]?
    let labels: [String: String]?

    var id: String { name }

    /// Docker labels these; older daemons only give them a 64-hex name.
    var isAnonymous: Bool {
        if labels?["com.docker.volume.anonymous"] != nil { return true }
        return name.count == 64 && name.allSatisfy { $0.isHexDigit && !$0.isUppercase }
    }
}

struct Network: Codable, Identifiable, Hashable {
    let id: String
    let name: String
    let driver: String
    let scope: String
    let created: Date?
    let `internal`: Bool
    let attachable: Bool
    let ipv6: Bool
    let containers: [String]
    let subnets: [String]?
    let labels: [String: String]?

    /// Docker's predefined networks always exist and cannot be removed.
    var isBuiltin: Bool { ["bridge", "host", "none", "ingress"].contains(name) }
}

struct Report: Codable {
    let schemaVersion: String
    let generatedAt: Date
    let tool: ToolInfo
    let docker: DockerInfo
    let score: Int
    let summary: Summary
    let findings: [Finding]
    let containers: [Container]?
    let images: [DockerImage]?
    let volumes: [Volume]?
    let networks: [Network]?
    let skippedRules: [String]?

    /// The most severe finding present, or nil when the environment is clean.
    var worstSeverity: Severity? {
        findings.map(\.severity).max()
    }

    /// Findings collapsed by rule, most severe first — the same grouping the
    /// CLI does, and for the same reason: fifteen containers running as root is
    /// one problem, not fifteen.
    var groupedFindings: [FindingGroup] {
        var order: [String] = []
        var groups: [String: FindingGroup] = [:]

        for finding in findings.sorted(by: { $0.severity > $1.severity }) {
            let key = "\(finding.ruleID)|\(finding.severity.rawValue)"
            if var existing = groups[key] {
                existing.findings.append(finding)
                groups[key] = existing
            } else {
                groups[key] = FindingGroup(
                    ruleID: finding.ruleID,
                    rule: finding.rule,
                    severity: finding.severity,
                    category: finding.category,
                    resource: finding.resource,
                    findings: [finding]
                )
                order.append(key)
            }
        }
        return order.compactMap { groups[$0] }
    }
}

struct FindingGroup: Identifiable, Hashable {
    let ruleID: String
    let rule: String
    let severity: Severity
    let category: Category
    let resource: ResourceKind
    var findings: [Finding]

    var id: String { "\(ruleID)|\(severity.rawValue)" }
    var count: Int { findings.count }

    var affectedLabel: String {
        count == 1 ? "1 \(resource.rawValue)" : "\(count) \(resource.rawValue)s"
    }

    var resourceNames: [String] { findings.map(\.resourceName) }
}
