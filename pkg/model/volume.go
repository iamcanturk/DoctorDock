package model

import "time"

// Volume is a normalized view of one Docker volume.
type Volume struct {
	Name       string    `json:"name"`
	Driver     string    `json:"driver"`
	Mountpoint string    `json:"mountpoint"`
	Scope      string    `json:"scope,omitempty"`
	Created    time.Time `json:"created,omitempty"`

	// Size is the volume's disk usage in bytes, or -1 when the daemon did not
	// report it. Computing it requires a disk-usage query that can be slow on
	// large environments, so it is only populated when explicitly requested.
	Size int64 `json:"size,omitempty"`

	// InUse means at least one container (running or stopped) mounts it.
	InUse bool `json:"in_use"`
	// UsedBy lists the names of containers mounting this volume.
	UsedBy []string `json:"used_by,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`
}

// AnonymousLabel is the marker Docker attaches to volumes it created
// implicitly for a container rather than ones a user named.
const AnonymousLabel = "com.docker.volume.anonymous"

// IsAnonymous reports whether the volume was created implicitly by a container
// rather than named by a user. These are the most common source of silent disk
// consumption, because nothing in a compose file or a run command names them.
//
// Recent daemons label them; older ones only give them a 64-character hex
// name, so both signals are checked.
func (v Volume) IsAnonymous() bool {
	if _, ok := v.Labels[AnonymousLabel]; ok {
		return true
	}
	if len(v.Name) != 64 {
		return false
	}
	for _, r := range v.Name {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			return false
		}
	}
	return true
}
