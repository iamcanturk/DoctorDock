package model

import (
	"strings"
	"time"
)

// Container state values as reported by the Docker daemon.
const (
	StateRunning    = "running"
	StateExited     = "exited"
	StateCreated    = "created"
	StateRestarting = "restarting"
	StatePaused     = "paused"
	StateRemoving   = "removing"
	StateDead       = "dead"
)

// Health status values. HealthNone means the container declares no healthcheck.
const (
	HealthNone      = "none"
	HealthStarting  = "starting"
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

// Container is a normalized view of one Docker container.
//
// This deliberately does not mirror the Docker SDK's shape. It carries exactly
// what rules and reports need, in a form that is stable across SDK versions.
type Container struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	// ImageID is the resolved image digest/ID, used to match against the image
	// list. Image (the reference) can be stale or absent after a re-tag.
	ImageID string `json:"image_id"`

	// State is one of the State* constants.
	State string `json:"state"`
	// Status is the daemon's human-readable status line, e.g. "Up 3 hours".
	Status  string    `json:"status"`
	Created time.Time `json:"created"`
	// StartedAt is zero for containers that have never run.
	StartedAt time.Time `json:"started_at,omitempty"`

	Ports    []Port   `json:"ports"`
	Mounts   []Mount  `json:"mounts"`
	Networks []string `json:"networks"`

	// RestartPolicy is the daemon's policy name: "no", "always",
	// "unless-stopped" or "on-failure". Empty is treated as "no".
	RestartPolicy string `json:"restart_policy"`
	// RestartCount is how many times the daemon has restarted this container.
	RestartCount int `json:"restart_count"`

	HasHealthcheck bool `json:"has_healthcheck"`
	// Health is one of the Health* constants.
	Health string `json:"health"`

	// User is the configured user. Empty means the image default was used,
	// which for most images is root.
	User string `json:"user"`
	// EffectiveUser resolves User against the image's configured user so that
	// rules do not have to. It is "root" when the container will run as uid 0.
	EffectiveUser string `json:"effective_user"`

	Privileged bool `json:"privileged"`
	// NetworkMode is the raw mode: "bridge", "host", "none",
	// "container:<id>" or a user-defined network name.
	NetworkMode string `json:"network_mode"`
	PidMode     string `json:"pid_mode"`
	IpcMode     string `json:"ipc_mode"`

	CapAdd  []string `json:"cap_add"`
	CapDrop []string `json:"cap_drop"`

	ReadOnlyRootFS bool `json:"read_only_rootfs"`

	// MemoryLimit is the hard memory limit in bytes. Zero means unlimited.
	MemoryLimit int64 `json:"memory_limit"`
	// NanoCPUs is the CPU quota in units of 1e-9 CPU. Zero means unlimited.
	NanoCPUs int64 `json:"nano_cpus"`
	// PidsLimit is the maximum process count. Zero or negative means unlimited.
	PidsLimit int64 `json:"pids_limit"`

	// EnvKeys holds environment variable *names only*. Values are discarded at
	// collection time and never enter this struct. See
	// docs/adr/0005-no-secret-collection.md.
	EnvKeys []string `json:"env_keys"`

	Labels map[string]string `json:"labels,omitempty"`
}

// IsRunning reports whether the container is currently executing.
func (c Container) IsRunning() bool { return c.State == StateRunning }

// IsUnhealthy reports whether a declared healthcheck is currently failing.
func (c Container) IsUnhealthy() bool { return c.Health == HealthUnhealthy }

// RunsAsRoot reports whether the container will execute as uid 0.
func (c Container) RunsAsRoot() bool {
	u := strings.TrimSpace(c.EffectiveUser)
	return u == "" || u == "root" || u == "0" || strings.HasPrefix(u, "0:")
}

// UsesHostNetwork reports whether the container shares the host network stack.
func (c Container) UsesHostNetwork() bool { return c.NetworkMode == "host" }

// ShortID returns the 12-character ID form that Docker displays.
func (c Container) ShortID() string { return shortID(c.ID) }

// Port is a published or exposed container port.
type Port struct {
	// PrivatePort is the port inside the container.
	PrivatePort uint16 `json:"private_port"`
	// PublicPort is the port on the host. Zero when the port is exposed but
	// not published.
	PublicPort uint16 `json:"public_port,omitempty"`
	// Type is "tcp", "udp" or "sctp".
	Type string `json:"type"`
	// HostIP is the interface the port is bound to. "0.0.0.0" or "::" means
	// every interface, which is what makes a published port reachable off-host.
	HostIP string `json:"host_ip,omitempty"`
}

// IsPublished reports whether the port is reachable from outside the container.
func (p Port) IsPublished() bool { return p.PublicPort != 0 }

// IsPublicallyBound reports whether the port is published on every interface
// rather than being restricted to loopback.
func (p Port) IsPublicallyBound() bool {
	if !p.IsPublished() {
		return false
	}
	switch p.HostIP {
	case "", "0.0.0.0", "::", "[::]":
		return true
	default:
		return false
	}
}

// Mount is a filesystem mount attached to a container.
type Mount struct {
	// Type is "bind", "volume", "tmpfs" or "npipe".
	Type string `json:"type"`
	// Source is the host path for bind mounts, or the volume name for volumes.
	Source string `json:"source"`
	// Destination is the path inside the container.
	Destination string `json:"destination"`
	// Name is the volume name, empty for bind mounts.
	Name     string `json:"name,omitempty"`
	ReadOnly bool   `json:"read_only"`
}

// IsBind reports whether the mount maps a host path directly into the container.
func (m Mount) IsBind() bool { return m.Type == "bind" }

// ShortID returns the 12-character identifier form Docker displays, with any
// "sha256:" prefix removed.
func ShortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func shortID(id string) string { return ShortID(id) }
