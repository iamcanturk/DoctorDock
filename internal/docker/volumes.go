package docker

import (
	"context"
	"fmt"
	"sort"

	"github.com/docker/docker/api/types/volume"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// ListVolumes returns every volume known to the daemon.
//
// Usage (InUse/UsedBy) is not resolved here — the scanner derives it from
// container mounts, because a volume's own record does not reliably say which
// containers reference it.
func (c *engineClient) ListVolumes(ctx context.Context) ([]model.Volume, error) {
	resp, err := c.api.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}

	out := make([]model.Volume, 0, len(resp.Volumes))
	for _, v := range resp.Volumes {
		if v == nil {
			continue
		}
		m := model.Volume{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Scope:      v.Scope,
			Created:    parseDockerTime(v.CreatedAt),
			Size:       -1,
			Labels:     v.Labels,
		}
		// UsageData is only populated by the disk-usage endpoint, which is
		// expensive; when the daemon does volunteer it, keep it.
		if v.UsageData != nil {
			m.Size = v.UsageData.Size
		}
		out = append(out, m)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
