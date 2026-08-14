import Foundation

// Realistic data for previews and the headless renderer, so views can be
// rasterised and inspected without a running daemon. Deliberately anonymised —
// the same discipline the share card enforces, applied to sample data too. Each
// rule is paired with its real severity so the preview icons are honest.
enum SampleData {
    /// A hardcoded explanation, so the detail design renders without the CLI.
    static let explanation = RuleExplanation(
        id: "DD005", name: "Docker socket exposed",
        severity: .critical, category: .security,
        description: "The Docker socket is mounted into the container.",
        explanation: .init(
            what: "The Docker socket (/var/run/docker.sock) is mounted into the container.",
            why: "Access to the Docker socket is root on the host. Anyone who can talk to it can start a new privileged container that mounts the host filesystem. Mounting it read-only does not help — it is an API endpoint, not a file.",
            scenario: "A container with the socket runs one API call to launch a second container with --privileged -v /:/host, then writes an SSH key into /host/root/.ssh/authorized_keys. Total elapsed time: seconds.",
            fixes: [
                .init(title: "Remove the mount", lang: "bash",
                      code: "# drop: -v /var/run/docker.sock:/var/run/docker.sock"),
                .init(title: "If the container needs the API, put a filtering proxy in front", lang: "yaml",
                      code: "services:\n  docker-proxy:\n    image: tecnativa/docker-socket-proxy\n    environment:\n      CONTAINERS: 1\n      POST: 0"),
            ],
            falsePositives: "None worth relying on. Watchtower, Portainer and Traefik all ask for the socket, and all are equivalent to giving that container host root.",
            references: [
                .init(title: "Docker socket security", url: "https://docs.docker.com/engine/security/protect-access/"),
                .init(title: "docker-socket-proxy", url: "https://github.com/Tecnativa/docker-socket-proxy"),
            ]),
        hasLongForm: true)

    static let sampleContainers: [Container] = (0..<8).map { i in
        let running: Bool = i < 5
        let state: String = running ? "running" : "exited"
        let status: String = running ? "Up 3 days" : "Exited (0) 2 days ago"
        let image: String = i % 2 == 0 ? "nginx:1.25-alpine" : "postgres:16"
        let ports: [Port] = i == 0
            ? [Port(privatePort: 5432, publicPort: 5432, type: "tcp", hostIP: "0.0.0.0")]
            : []
        let health: String = i == 2 ? "unhealthy" : (i < 3 ? "healthy" : "none")
        let user: String = i % 2 == 0 ? "root" : "postgres"
        let restarts: Int = i == 7 ? 12 : 0
        return Container(
            id: "c\(i)", name: "service-\(i)", image: image, imageId: "sha256:img\(i)",
            state: state, status: status,
            created: Date(timeIntervalSince1970: 1_770_000_000), startedAt: nil,
            ports: ports, mounts: [], networks: ["app_net"],
            restartPolicy: "unless-stopped", restartCount: restarts,
            hasHealthcheck: i < 3, health: health,
            user: "", effectiveUser: user, privileged: false,
            networkMode: "app_net", pidMode: "", ipcMode: "private",
            capAdd: [], capDrop: [], readOnlyRootfs: false,
            memoryLimit: 0, nanoCpus: 0, pidsLimit: 0, envKeys: ["PATH"], labels: nil)
    }

    static let sampleImages: [DockerImage] = (0..<6).map { i in
        let dangling: Bool = i >= 4
        let inUse: Bool = i < 2
        let tags: [String] = dangling ? [] : ["app-\(i):1.2.\(i)"]
        let usedBy: [String]? = inUse ? ["service-\(i)"] : nil
        let size: Int64 = Int64(50_000_000) * Int64(i + 1)
        return DockerImage(
            id: "sha256:image\(i)abcdef", repoTags: tags, repoDigests: nil,
            size: size, sharedSize: -1,
            created: Date(timeIntervalSince1970: 1_760_000_000),
            architecture: nil, os: nil, layers: nil,
            dangling: dangling, inUse: inUse, usedBy: usedBy, labels: nil)
    }

    static let sampleVolumes: [Volume] = (0..<5).map { i in
        let inUse: Bool = i < 2
        let name: String = inUse ? "app-data-\(i)" : String(repeating: "a", count: 64)
        let usedBy: [String]? = inUse ? ["service-\(i)"] : nil
        return Volume(
            name: name, driver: "local",
            mountpoint: "/var/lib/docker/volumes/v\(i)/_data", scope: "local",
            created: nil, size: nil, inUse: inUse, usedBy: usedBy, labels: nil)
    }

    static let sampleNetworks: [Network] = (0..<5).map { i in
        let octet: Int = 20 + i
        let name: String = i == 0 ? "bridge" : "project-\(i)_default"
        let attached: [String] = i == 1 ? ["service-0", "service-1"] : []
        return Network(
            id: "net\(i)", name: name, driver: "bridge", scope: "local",
            created: nil, internal: false, attachable: false, ipv6: false,
            containers: attached, subnets: ["172.\(octet).0.0/16"], labels: nil)
    }

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
            containers: sampleContainers, images: sampleImages,
            volumes: sampleVolumes, networks: sampleNetworks,
            skippedRules: nil)
    }()
}
