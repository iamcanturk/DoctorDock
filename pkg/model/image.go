package model

import (
	"strings"
	"time"
)

// Image is a normalized view of one Docker image.
type Image struct {
	ID string `json:"id"`
	// RepoTags holds references such as "nginx:1.25". Empty for dangling images.
	RepoTags []string `json:"repo_tags"`
	// RepoDigests holds digest references such as "nginx@sha256:...".
	RepoDigests []string `json:"repo_digests,omitempty"`

	// Size is the image size in bytes as reported by the daemon.
	Size int64 `json:"size"`
	// SharedSize is the number of bytes shared with other images, or -1 when
	// the daemon did not compute it.
	SharedSize int64 `json:"shared_size,omitempty"`

	Created time.Time `json:"created"`

	Architecture string `json:"architecture,omitempty"`
	OS           string `json:"os,omitempty"`
	// Layers is the number of filesystem layers, or 0 when unknown.
	Layers int `json:"layers,omitempty"`

	// Dangling means the image has no repository tag: usually a leftover from
	// a rebuild that re-pointed the tag to a new image.
	Dangling bool `json:"dangling"`
	// InUse means at least one container (running or stopped) references it.
	InUse bool `json:"in_use"`
	// UsedBy lists the names of containers referencing this image.
	UsedBy []string `json:"used_by,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`
}

// PrimaryTag returns the first repository tag, or "<none>:<none>" for a
// dangling image.
func (i Image) PrimaryTag() string {
	for _, t := range i.RepoTags {
		if t != "" && t != "<none>:<none>" {
			return t
		}
	}
	return "<none>:<none>"
}

// DisplayName returns the best human-readable identifier: a tag if the image
// has one, otherwise the short ID.
func (i Image) DisplayName() string {
	if tag := i.PrimaryTag(); tag != "<none>:<none>" {
		return tag
	}
	return i.ShortID()
}

// ShortID returns the 12-character ID form that Docker displays.
func (i Image) ShortID() string { return shortID(i.ID) }

// Age returns how long ago the image was created, relative to now.
func (i Image) Age(now time.Time) time.Duration {
	if i.Created.IsZero() {
		return 0
	}
	return now.Sub(i.Created)
}

// HasMutableTag reports whether any of the image's tags is a moving target
// such as :latest, which makes deployments non-reproducible.
func (i Image) HasMutableTag() bool {
	for _, t := range i.RepoTags {
		if IsMutableTag(t) {
			return true
		}
	}
	return false
}

// IsMutableTag reports whether a reference points at a tag whose meaning can
// change under it. An untagged reference implies :latest.
func IsMutableTag(ref string) bool {
	if ref == "" || ref == "<none>:<none>" {
		return false
	}
	// A digest reference is immutable by construction.
	if strings.Contains(ref, "@sha256:") {
		return false
	}
	tag := TagOf(ref)
	switch tag {
	case "", "latest", "main", "master", "edge", "stable", "dev", "nightly":
		return true
	default:
		return false
	}
}

// TagOf extracts the tag portion of an image reference, returning "" when the
// reference carries no tag. It accounts for registry references that include a
// port, such as "registry.local:5000/app".
func TagOf(ref string) string {
	if at := strings.Index(ref, "@"); at != -1 {
		ref = ref[:at]
	}
	colon := strings.LastIndex(ref, ":")
	if colon == -1 {
		return ""
	}
	// A colon before the last slash belongs to a registry host:port, not a tag.
	if slash := strings.LastIndex(ref, "/"); slash > colon {
		return ""
	}
	return ref[colon+1:]
}

// RepositoryOf extracts the repository portion of an image reference,
// stripping any tag or digest.
func RepositoryOf(ref string) string {
	if at := strings.Index(ref, "@"); at != -1 {
		ref = ref[:at]
	}
	colon := strings.LastIndex(ref, ":")
	if colon == -1 {
		return ref
	}
	if slash := strings.LastIndex(ref, "/"); slash > colon {
		return ref
	}
	return ref[:colon]
}
