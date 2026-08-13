package model

import "time"

// Docker's four predefined networks, which always exist and must never be
// reported as unused.
var builtinNetworks = map[string]bool{
	"bridge":  true,
	"host":    true,
	"none":    true,
	"ingress": true,
}

// Network is a normalized view of one Docker network.
type Network struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`

	Created    time.Time `json:"created,omitempty"`
	Internal   bool      `json:"internal"`
	Attachable bool      `json:"attachable"`
	IPv6       bool      `json:"ipv6"`

	// Containers lists the names of containers attached to this network.
	Containers []string `json:"containers"`

	// Subnets holds the configured CIDR ranges.
	Subnets []string `json:"subnets,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`
}

// IsBuiltin reports whether this is one of Docker's predefined networks, which
// cannot be removed and must never be flagged as unused.
func (n Network) IsBuiltin() bool { return builtinNetworks[n.Name] }

// InUse reports whether any container is attached.
func (n Network) InUse() bool { return len(n.Containers) > 0 }

// ShortID returns the 12-character ID form that Docker displays.
func (n Network) ShortID() string { return shortID(n.ID) }
