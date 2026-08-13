package docker

import (
	"context"
	"fmt"
	"sort"

	"github.com/docker/docker/api/types/network"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// ListNetworks returns every network known to the daemon.
//
// Container attachments are deliberately left empty. The daemon's network list
// endpoint does not populate them, and inspecting every network with Verbose
// would be a second round of API calls for information the container list
// already contains. The scanner resolves attachments from container network
// settings instead.
func (c *engineClient) ListNetworks(ctx context.Context) ([]model.Network, error) {
	summaries, err := c.api.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}

	out := make([]model.Network, 0, len(summaries))
	for _, n := range summaries {
		m := model.Network{
			ID:         n.ID,
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			Created:    n.Created.UTC(),
			Internal:   n.Internal,
			Attachable: n.Attachable,
			IPv6:       n.EnableIPv6,
			Containers: []string{},
			Labels:     n.Labels,
		}
		for _, cfg := range n.IPAM.Config {
			if cfg.Subnet != "" {
				m.Subnets = append(m.Subnets, cfg.Subnet)
			}
		}
		out = append(out, m)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
