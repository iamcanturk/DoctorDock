package model

import "time"

// DockerInfo describes the daemon DoctorDock is talking to.
type DockerInfo struct {
	ServerVersion string `json:"server_version"`
	APIVersion    string `json:"api_version"`
	// OSType is "linux" or "windows" — the container platform, which is not
	// necessarily the platform DoctorDock itself runs on.
	OSType        string `json:"os_type"`
	Architecture  string `json:"architecture"`
	KernelVersion string `json:"kernel_version,omitempty"`
	// OperatingSystem is the daemon host's OS description, e.g.
	// "Docker Desktop" or "Ubuntu 24.04.1 LTS".
	OperatingSystem string `json:"operating_system,omitempty"`
	StorageDriver   string `json:"storage_driver,omitempty"`
	CgroupVersion   string `json:"cgroup_version,omitempty"`
	CPUs            int    `json:"cpus,omitempty"`
	MemTotal        int64  `json:"mem_total,omitempty"`
	// Rootless reports whether the daemon runs without root privileges, which
	// changes how seriously several container-level findings should be taken.
	Rootless bool `json:"rootless"`
	// SecurityOptions lists daemon-level security features such as
	// "seccomp", "apparmor" or "userns".
	SecurityOptions []string `json:"security_options,omitempty"`
}

// Environment is a point-in-time snapshot of everything DoctorDock collected.
// It is the input to every rule.
//
// Usage relationships between resources (which images are referenced, which
// volumes are mounted, which networks are attached) are resolved once by the
// scanner before rules run, so rules read a boolean instead of cross-scanning
// the whole snapshot. See docs/adr/0003-whole-environment-rule-target.md.
type Environment struct {
	CollectedAt time.Time  `json:"collected_at"`
	Docker      DockerInfo `json:"docker"`

	Containers []Container `json:"containers"`
	Images     []Image     `json:"images"`
	Volumes    []Volume    `json:"volumes"`
	Networks   []Network   `json:"networks"`
}

// RunningContainers returns only the containers currently executing.
func (e *Environment) RunningContainers() []Container {
	out := make([]Container, 0, len(e.Containers))
	for _, c := range e.Containers {
		if c.IsRunning() {
			out = append(out, c)
		}
	}
	return out
}

// ContainerSummary counts containers by state.
type ContainerSummary struct {
	Total      int `json:"total"`
	Running    int `json:"running"`
	Stopped    int `json:"stopped"`
	Paused     int `json:"paused"`
	Restarting int `json:"restarting"`
	Created    int `json:"created"`
	Unhealthy  int `json:"unhealthy"`
}

// ImageSummary counts images and their disk usage.
type ImageSummary struct {
	Total    int `json:"total"`
	Dangling int `json:"dangling"`
	// Unused counts images no container references. Dangling images are also
	// unused, and are counted in both fields.
	Unused int `json:"unused"`
	// TotalSize is the sum of all image sizes in bytes. Because images share
	// layers, this over-counts actual disk usage.
	TotalSize int64 `json:"total_size"`
	// ReclaimableSize is the summed size of unused images in bytes: an upper
	// bound on what a prune would free.
	ReclaimableSize int64 `json:"reclaimable_size"`
}

// VolumeSummary counts volumes.
type VolumeSummary struct {
	Total     int `json:"total"`
	Unused    int `json:"unused"`
	Anonymous int `json:"anonymous"`
}

// NetworkSummary counts networks.
type NetworkSummary struct {
	Total int `json:"total"`
	// Custom counts networks other than Docker's predefined ones.
	Custom int `json:"custom"`
	// Unused counts custom networks with no attached containers. Predefined
	// networks are never counted as unused.
	Unused int `json:"unused"`
}

// FindingSummary aggregates findings for at-a-glance consumption.
type FindingSummary struct {
	Total      int              `json:"total"`
	BySeverity SeverityCounts   `json:"by_severity"`
	ByCategory map[Category]int `json:"by_category"`
}

// Summary is the aggregate view of an environment plus its findings. Clients
// that only need headline numbers can read this and ignore the resource lists.
type Summary struct {
	Containers ContainerSummary `json:"containers"`
	Images     ImageSummary     `json:"images"`
	Volumes    VolumeSummary    `json:"volumes"`
	Networks   NetworkSummary   `json:"networks"`
	Findings   FindingSummary   `json:"findings"`
}

// Summarize computes aggregate counts for an environment and a set of findings.
func Summarize(env *Environment, findings []Finding) Summary {
	var s Summary

	s.Containers.Total = len(env.Containers)
	for _, c := range env.Containers {
		switch c.State {
		case StateRunning:
			s.Containers.Running++
		case StatePaused:
			s.Containers.Paused++
		case StateRestarting:
			s.Containers.Restarting++
		case StateCreated:
			s.Containers.Created++
		default:
			// exited, dead, removing — all "not running, not transitioning".
			s.Containers.Stopped++
		}
		// A stopped container keeps its last health status, so counting those
		// would report containers that were unhealthy days ago as unhealthy now.
		if c.IsRunning() && c.IsUnhealthy() {
			s.Containers.Unhealthy++
		}
	}

	s.Images.Total = len(env.Images)
	for _, img := range env.Images {
		s.Images.TotalSize += img.Size
		if img.Dangling {
			s.Images.Dangling++
		}
		if !img.InUse {
			s.Images.Unused++
			s.Images.ReclaimableSize += img.Size
		}
	}

	s.Volumes.Total = len(env.Volumes)
	for _, v := range env.Volumes {
		if !v.InUse {
			s.Volumes.Unused++
		}
		if v.IsAnonymous() {
			s.Volumes.Anonymous++
		}
	}

	s.Networks.Total = len(env.Networks)
	for _, n := range env.Networks {
		if n.IsBuiltin() {
			continue
		}
		s.Networks.Custom++
		if !n.InUse() {
			s.Networks.Unused++
		}
	}

	s.Findings.Total = len(findings)
	s.Findings.BySeverity = CountSeverities(findings)
	s.Findings.ByCategory = make(map[Category]int, len(AllCategories))
	for _, cat := range AllCategories {
		s.Findings.ByCategory[cat] = 0
	}
	for _, f := range findings {
		s.Findings.ByCategory[f.Category]++
	}

	return s
}
