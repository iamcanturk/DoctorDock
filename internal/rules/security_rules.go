package rules

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// --- DD001 ------------------------------------------------------------------

// RootUser reports containers whose processes start as uid 0.
type RootUser struct{}

func (RootUser) ID() string               { return "DD001" }
func (RootUser) Name() string             { return "Container runs as root" }
func (RootUser) Category() model.Category { return model.CategorySecurity }
func (RootUser) Severity() model.Severity { return model.SeverityHigh }
func (RootUser) Description() string {
	return "Reports containers that start as uid 0, either because no USER is set on the container " +
		"or because the image itself does not set one."
}

func (r RootUser) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		if !c.RunsAsRoot() {
			continue
		}
		f := newContainerFinding(r, c)
		f.Title = "Container runs as root"
		f.Description = "Processes in this container start as uid 0. If the application is " +
			"compromised, the attacker begins with root inside the container, which makes every " +
			"subsequent container-escape technique easier. Note that an entrypoint which drops " +
			"privileges at runtime cannot be detected from configuration alone."
		f.Recommendation = "Add a non-root USER to the image, or start the container with " +
			"`--user 1000:1000` (compose: `user: \"1000:1000\"`). For official images that need " +
			"root to initialise, check whether the image already supports a non-root mode."
		// An unset effective user means the image set no USER either, which
		// resolves to root — report what it actually is, not a blank.
		effective := strings.TrimSpace(c.EffectiveUser)
		if effective == "" {
			effective = "root"
		}
		f.Details = map[string]string{
			"configured_user": orNone(c.User),
			"effective_user":  effective,
			"image":           c.Image,
		}
		out = append(out, f)
	}
	return out
}

// --- DD002 ------------------------------------------------------------------

// PrivilegedContainer reports containers running with --privileged.
type PrivilegedContainer struct{}

func (PrivilegedContainer) ID() string               { return "DD002" }
func (PrivilegedContainer) Name() string             { return "Privileged container" }
func (PrivilegedContainer) Category() model.Category { return model.CategorySecurity }
func (PrivilegedContainer) Severity() model.Severity { return model.SeverityCritical }
func (PrivilegedContainer) Description() string {
	return "Reports containers started with --privileged, which grants all capabilities and " +
		"removes device and namespace restrictions."
}

func (r PrivilegedContainer) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		if !c.Privileged {
			continue
		}
		f := newContainerFinding(r, c)
		f.Title = "Container is running in privileged mode"
		f.Description = "Privileged mode grants every Linux capability, disables the seccomp and " +
			"AppArmor profiles, and gives the container access to all host devices. Escaping to " +
			"the host from a privileged container is trivial and well documented."
		f.Recommendation = "Remove `--privileged` (compose: `privileged: true`). If the workload " +
			"genuinely needs elevated access, grant only the specific capabilities it requires " +
			"with `--cap-add`, or mount only the specific device with `--device`."
		out = append(out, f)
	}
	return out
}

// --- DD003 ------------------------------------------------------------------

// HostNetwork reports containers sharing the host network namespace.
type HostNetwork struct{}

func (HostNetwork) ID() string               { return "DD003" }
func (HostNetwork) Name() string             { return "Host networking" }
func (HostNetwork) Category() model.Category { return model.CategorySecurity }
func (HostNetwork) Severity() model.Severity { return model.SeverityMedium }
func (HostNetwork) Description() string {
	return "Reports containers using --network=host, which removes network isolation between the " +
		"container and the host."
}

func (r HostNetwork) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		if !c.UsesHostNetwork() {
			continue
		}
		f := newContainerFinding(r, c)
		f.Title = "Container uses host networking"
		f.Description = "The container shares the host's network namespace. Every port it opens is " +
			"bound directly on the host with no port-mapping layer, it can reach services listening " +
			"on the host's loopback interface, and it can observe host network traffic."
		f.Recommendation = "Use a user-defined bridge network and publish only the ports you need " +
			"with `-p`. Host networking is rarely required outside of high-throughput proxies and " +
			"network monitoring tools."
		out = append(out, f)
	}
	return out
}

// --- DD004 ------------------------------------------------------------------

// sensitiveHostPaths maps a host path to why mounting it is dangerous.
// Matching is prefix-based on the normalized path.
var sensitiveHostPaths = []struct {
	path   string
	reason string
}{
	{"/", "the entire host filesystem"},
	{"/etc", "host system configuration, including users, passwords and service definitions"},
	{"/root", "the host root user's home directory"},
	{"/boot", "the host bootloader and kernel images"},
	{"/dev", "host device nodes, including raw disks"},
	{"/proc", "host process and kernel state"},
	{"/sys", "host kernel and hardware interfaces, including cgroup controls"},
	{"/var/run", "host runtime sockets"},
	{"/run", "host runtime sockets"},
	{"/var/lib/docker", "Docker's own state, including every other container's filesystem"},
	{"/usr", "host system binaries and libraries"},
	{"/bin", "host system binaries"},
	{"/sbin", "host system binaries"},
	{"/lib", "host system libraries"},
	{"/var/lib/kubelet", "Kubernetes node state"},
	{"/etc/kubernetes", "Kubernetes cluster configuration and credentials"},
}

// sensitiveHomeDirs are credential directories inside a user's home, matched
// on the trailing path segment so they work regardless of the user name.
var sensitiveHomeDirs = map[string]string{
	".ssh":           "SSH private keys",
	".aws":           "AWS credentials",
	".kube":          "Kubernetes cluster credentials",
	".docker":        "Docker registry credentials",
	".gnupg":         "GPG private keys",
	".config/gcloud": "Google Cloud credentials",
	".azure":         "Azure credentials",
}

// SensitiveHostMount reports bind mounts that expose sensitive host paths.
type SensitiveHostMount struct{}

func (SensitiveHostMount) ID() string               { return "DD004" }
func (SensitiveHostMount) Name() string             { return "Sensitive host path mounted" }
func (SensitiveHostMount) Category() model.Category { return model.CategorySecurity }
func (SensitiveHostMount) Severity() model.Severity { return model.SeverityHigh }
func (SensitiveHostMount) Description() string {
	return "Reports bind mounts of host paths that expose system state or credentials, such as /etc, " +
		"/var/lib/docker or ~/.ssh."
}

func (r SensitiveHostMount) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		for _, m := range c.Mounts {
			if !m.IsBind() {
				continue
			}
			hostPath := normalizeHostPath(m.Source)

			// The Docker socket has its own rule; reporting it twice would
			// double-count the same exposure.
			if isDockerSocket(hostPath, m.Destination) {
				continue
			}

			reason, matched := matchSensitivePath(hostPath)
			if !matched {
				continue
			}

			f := newContainerFinding(r, c)
			// Mounting the host root writable is not a lesser version of
			// mounting /etc — it is total host compromise, so it is reported
			// at the level it actually represents.
			if hostPath == "/" && !m.ReadOnly {
				f.Severity = model.SeverityCritical
			}
			f.Title = fmt.Sprintf("Container mounts sensitive host path %s", hostPath)
			f.Description = fmt.Sprintf(
				"The host path %s is bind-mounted at %s (%s). This exposes %s to the container.",
				hostPath, m.Destination, readWriteLabel(m.ReadOnly), reason)
			f.Recommendation = fmt.Sprintf(
				"Remove the mount, narrow it to the specific file or subdirectory the container "+
					"actually needs, or at minimum make it read-only (`%s:%s:ro`).",
				hostPath, m.Destination)
			f.Details = map[string]string{
				"host_path":   hostPath,
				"destination": m.Destination,
				"read_only":   fmt.Sprintf("%t", m.ReadOnly),
			}
			out = append(out, f)
		}
	}
	return out
}

// matchSensitivePath reports whether a host path is sensitive, and why.
func matchSensitivePath(hostPath string) (string, bool) {
	if hostPath == "/" {
		return sensitiveHostPaths[0].reason, true
	}

	for _, s := range sensitiveHostPaths {
		if s.path == "/" {
			continue
		}
		if hostPath == s.path || strings.HasPrefix(hostPath, s.path+"/") {
			return s.reason, true
		}
	}

	// Credential directories live under an arbitrary home directory, so match
	// on the path suffix instead of a fixed prefix.
	for dir, reason := range sensitiveHomeDirs {
		if hostPath == "/"+dir || strings.HasSuffix(hostPath, "/"+dir) ||
			strings.Contains(hostPath, "/"+dir+"/") {
			return reason, true
		}
	}

	return "", false
}

// normalizeHostPath strips the prefix Docker Desktop adds to macOS and Windows
// bind sources. Without this, /Users/me/.ssh arrives as /host_mnt/Users/me/.ssh
// and no sensitive-path rule would ever match on a Mac.
func normalizeHostPath(p string) string {
	if p == "" {
		return p
	}
	for _, prefix := range []string{"/host_mnt", "/run/desktop/mnt/host"} {
		if strings.HasPrefix(p, prefix) {
			trimmed := strings.TrimPrefix(p, prefix)
			if trimmed == "" {
				return "/"
			}
			if strings.HasPrefix(trimmed, "/") {
				p = trimmed
			}
			break
		}
	}
	return path.Clean(p)
}

// --- DD005 ------------------------------------------------------------------

// DockerSocketMount reports containers with access to the Docker socket.
type DockerSocketMount struct{}

func (DockerSocketMount) ID() string               { return "DD005" }
func (DockerSocketMount) Name() string             { return "Docker socket exposed" }
func (DockerSocketMount) Category() model.Category { return model.CategorySecurity }
func (DockerSocketMount) Severity() model.Severity { return model.SeverityCritical }
func (DockerSocketMount) Description() string {
	return "Reports containers with the Docker socket mounted, which is equivalent to giving them " +
		"root on the host."
}

func (r DockerSocketMount) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		for _, m := range c.Mounts {
			if !isDockerSocket(normalizeHostPath(m.Source), m.Destination) {
				continue
			}
			f := newContainerFinding(r, c)
			f.Title = "Docker socket is mounted into the container"
			f.Description = "Access to the Docker socket is equivalent to root on the host. Anyone " +
				"who can talk to it can start a new privileged container that mounts the host " +
				"filesystem — no kernel exploit required. Read-only does not help, because the " +
				"socket is an API endpoint, not a file."
			f.Recommendation = "Remove the socket mount. If the container needs the Docker API, put " +
				"a filtering proxy such as docker-socket-proxy in front of it and grant only the " +
				"endpoints required, or use a rootless/sysbox runtime."
			f.Details = map[string]string{
				"host_path":   m.Source,
				"destination": m.Destination,
				"read_only":   fmt.Sprintf("%t", m.ReadOnly),
			}
			out = append(out, f)
		}
	}
	return out
}

func isDockerSocket(hostPath, destination string) bool {
	return strings.HasSuffix(hostPath, "docker.sock") ||
		strings.HasSuffix(destination, "docker.sock") ||
		strings.EqualFold(hostPath, `\\.\pipe\docker_engine`) ||
		strings.EqualFold(destination, `\\.\pipe\docker_engine`)
}

// --- DD006 ------------------------------------------------------------------

// sensitivePorts maps a container-internal port to the service it identifies.
// The private port is used because it names the service regardless of which
// host port it was published on.
var sensitivePorts = map[uint16]string{
	22:    "SSH",
	23:    "Telnet",
	25:    "SMTP",
	445:   "SMB",
	1433:  "Microsoft SQL Server",
	1521:  "Oracle Database",
	2181:  "ZooKeeper",
	2375:  "Docker API (unencrypted)",
	2376:  "Docker API (TLS)",
	3306:  "MySQL/MariaDB",
	3389:  "RDP",
	4444:  "Selenium/metasploit-style service",
	5432:  "PostgreSQL",
	5672:  "RabbitMQ (AMQP)",
	5900:  "VNC",
	5984:  "CouchDB",
	6379:  "Redis",
	6443:  "Kubernetes API server",
	7001:  "Cassandra (internode)",
	8086:  "InfluxDB",
	9042:  "Cassandra (CQL)",
	9092:  "Kafka",
	9200:  "Elasticsearch",
	9300:  "Elasticsearch (transport)",
	10250: "kubelet API",
	11211: "Memcached",
	15672: "RabbitMQ management UI",
	27017: "MongoDB",
	27018: "MongoDB (shard)",
}

// ExposedSensitivePort reports datastore and admin ports published to every
// network interface.
type ExposedSensitivePort struct{}

func (ExposedSensitivePort) ID() string               { return "DD006" }
func (ExposedSensitivePort) Name() string             { return "Sensitive port exposed" }
func (ExposedSensitivePort) Category() model.Category { return model.CategorySecurity }
func (ExposedSensitivePort) Severity() model.Severity { return model.SeverityMedium }
func (ExposedSensitivePort) Description() string {
	return "Reports database, message-broker and admin ports published on all interfaces rather " +
		"than bound to loopback."
}

func (r ExposedSensitivePort) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		for _, p := range c.Ports {
			// A port bound to 127.0.0.1 is only reachable from the host, which
			// is the recommended way to expose a database for local work.
			if !p.IsPublicallyBound() {
				continue
			}
			service, sensitive := sensitivePorts[p.PrivatePort]
			if !sensitive {
				continue
			}

			f := newContainerFinding(r, c)
			// The Docker API on a wildcard address is remote code execution as
			// root for anyone who can route to the host.
			if p.PrivatePort == 2375 || p.PrivatePort == 2376 {
				f.Severity = model.SeverityCritical
			}
			f.Title = fmt.Sprintf("%s port %d is published on all interfaces", service, p.PrivatePort)
			f.Description = fmt.Sprintf(
				"Host port %d is bound to 0.0.0.0 and forwards to %s inside the container. Anything "+
					"that can route to this host can reach the service, including other machines on "+
					"the same network and, on a cloud host without a firewall, the internet.",
				p.PublicPort, service)
			f.Recommendation = fmt.Sprintf(
				"Bind the publish to loopback (`-p 127.0.0.1:%d:%d`) if only the host needs access, "+
					"or drop the publish entirely and let other containers reach it over a shared "+
					"Docker network.",
				p.PublicPort, p.PrivatePort)
			f.Details = map[string]string{
				"service":        service,
				"host_port":      fmt.Sprintf("%d", p.PublicPort),
				"container_port": fmt.Sprintf("%d", p.PrivatePort),
				"protocol":       p.Type,
			}
			out = append(out, f)
		}
	}
	return out
}

// --- DD009 ------------------------------------------------------------------

// dangerousCapabilities maps an added capability to what it allows. Values are
// phrased to complete the sentence "allows the container to ...".
var dangerousCapabilities = map[string]string{
	"ALL":             "do anything a privileged container can",
	"SYS_ADMIN":       "mount filesystems and manipulate namespaces, the most common container-escape primitive",
	"SYS_MODULE":      "load kernel modules, which is unrestricted host code execution",
	"SYS_RAWIO":       "access raw I/O ports and memory",
	"SYS_PTRACE":      "attach to and read the memory of other processes",
	"DAC_READ_SEARCH": "bypass file read permission checks and open host files by inode",
	"BPF":             "load eBPF programs into the kernel",
	"PERFMON":         "read kernel performance data across the host",
	"NET_ADMIN":       "reconfigure host networking, including firewall rules and interfaces",
	"SYS_BOOT":        "reboot the host",
	"SYS_TIME":        "change the host clock",
}

// criticalCapabilities are the ones that amount to host compromise on their own.
var criticalCapabilities = map[string]bool{
	"ALL":        true,
	"SYS_ADMIN":  true,
	"SYS_MODULE": true,
	"SYS_RAWIO":  true,
}

// DangerousCapabilities reports containers granted capabilities that undermine
// container isolation.
type DangerousCapabilities struct{}

func (DangerousCapabilities) ID() string               { return "DD009" }
func (DangerousCapabilities) Name() string             { return "Dangerous capabilities added" }
func (DangerousCapabilities) Category() model.Category { return model.CategorySecurity }
func (DangerousCapabilities) Severity() model.Severity { return model.SeverityHigh }
func (DangerousCapabilities) Description() string {
	return "Reports containers granted Linux capabilities that weaken or defeat container isolation, " +
		"such as SYS_ADMIN or SYS_MODULE."
}

func (r DangerousCapabilities) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		// A privileged container has every capability by definition; DD002
		// already reports it at CRITICAL, so repeating it here is noise.
		if c.Privileged {
			continue
		}

		var granted []string
		critical := false
		for _, capName := range c.CapAdd {
			if _, dangerous := dangerousCapabilities[capName]; !dangerous {
				continue
			}
			granted = append(granted, capName)
			if criticalCapabilities[capName] {
				critical = true
			}
		}
		if len(granted) == 0 {
			continue
		}
		sort.Strings(granted)

		reasons := make([]string, 0, len(granted))
		for _, capName := range granted {
			reasons = append(reasons, fmt.Sprintf("%s (%s)", capName, dangerousCapabilities[capName]))
		}

		f := newContainerFinding(r, c)
		if critical {
			f.Severity = model.SeverityCritical
		}
		f.Title = fmt.Sprintf("Container adds dangerous capabilities: %s", strings.Join(granted, ", "))
		f.Description = "Added capabilities let the container " + joinWithAnd(reasons) + "."
		f.Recommendation = "Drop the capability if the workload does not need it. If it does, " +
			"consider `--cap-drop=ALL` followed by adding back only the narrow capabilities required."
		f.Details = map[string]string{"capabilities": strings.Join(granted, ",")}
		out = append(out, f)
	}
	return out
}

func joinWithAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}

func readWriteLabel(readOnly bool) string {
	if readOnly {
		return "read-only"
	}
	return "read-write"
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "<not set>"
	}
	return s
}
