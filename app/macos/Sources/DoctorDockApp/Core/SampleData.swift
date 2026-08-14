import Foundation

// Realistic data for previews and the headless renderer, so views can be
// rasterised and inspected without a running daemon. Deliberately anonymised —
// the same discipline the share card enforces, applied to sample data too. Each
// rule is paired with its real severity so the preview icons are honest.
enum SampleData {
    static let report: Report = {
        // (ruleID, name, category, resource, severity, count)
        let groups: [(String, String, Category, ResourceKind, Severity, Int)] = [
            ("DD005", "Docker socket exposed", .security, .container, .critical, 1),
            ("DD001", "Container runs as root", .security, .container, .high, 6),
            ("DD006", "Sensitive port exposed", .security, .container, .medium, 4),
            ("DD007", "No healthcheck", .configuration, .container, .low, 8),
            ("DD015", "Unused image", .cleanup, .image, .info, 12),
        ]

        var findings: [Finding] = []
        for (id, name, cat, res, sev, count) in groups {
            for i in 0..<count {
                findings.append(Finding(
                    id_: id, rule: name, severity: sev, category: cat, resource: res,
                    resourceId: "\(id)-\(i)", resourceName: "service-\(i)",
                    title: name, description: "Sample description.",
                    recommendation: "Sample recommendation.", details: nil))
            }
        }

        let counts = SeverityCounts(info: 12, low: 8, medium: 4, high: 6, critical: 1)

        return Report(
            schemaVersion: "1.0",
            generatedAt: Date(timeIntervalSince1970: 1_776_000_000),
            tool: ToolInfo(name: "doctordock", version: "0.1.0", commit: "abc1234"),
            docker: DockerInfo(
                serverVersion: "29.2.1", apiVersion: "1.51", osType: "linux",
                architecture: "aarch64", kernelVersion: "6.12", operatingSystem: "Docker Desktop",
                storageDriver: "overlayfs", cgroupVersion: "2", cpus: 8,
                memTotal: 8_217_600_000, rootless: false, securityOptions: ["seccomp"]),
            score: 42,
            summary: Summary(
                containers: ContainerSummary(total: 26, running: 8, stopped: 17,
                    paused: 0, restarting: 1, created: 0, unhealthy: 2),
                images: ImageSummary(total: 32, dangling: 2, unused: 10,
                    totalSize: 13_468_031_315, reclaimableSize: 4_144_988_878),
                volumes: VolumeSummary(total: 29, unused: 18, anonymous: 13),
                networks: NetworkSummary(total: 12, custom: 9, unused: 3),
                findings: FindingSummary(total: counts.total,
                    bySeverity: counts,
                    byCategory: ["SECURITY": 11, "CONFIGURATION": 8, "CLEANUP": 12])),
            findings: findings,
            containers: [], images: [], volumes: [], networks: [],
            skippedRules: nil)
    }()
}
