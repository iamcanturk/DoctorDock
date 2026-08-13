package docker

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/docker/docker/api/types/image"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// ListImages returns every top-level image, including dangling ones.
//
// Intermediate build layers are excluded (All: false), matching what
// `docker images` shows — they are not actionable for a user.
func (c *engineClient) ListImages(ctx context.Context) ([]model.Image, error) {
	summaries, err := c.api.ImageList(ctx, image.ListOptions{SharedSize: true})
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	out := make([]model.Image, len(summaries))
	forEach(len(summaries), func(i int) {
		out[i] = c.buildImage(ctx, summaries[i])
	})

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].DisplayName(), out[j].DisplayName()
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (c *engineClient) buildImage(ctx context.Context, s image.Summary) model.Image {
	m := model.Image{
		ID:          s.ID,
		RepoTags:    cleanRefs(s.RepoTags),
		RepoDigests: cleanRefs(s.RepoDigests),
		Size:        s.Size,
		SharedSize:  s.SharedSize,
		Created:     time.Unix(s.Created, 0).UTC(),
		Labels:      s.Labels,
	}
	// Dangling matches Docker's own definition — no references of any kind,
	// neither tags nor digests — because DD014 tells the user to run
	// `docker image prune`, which removes exactly what `dangling=true` matches.
	// Flagging an image that prune would leave behind sends them after a
	// command that does nothing.
	//
	// An image that kept a digest reference but lost its tag is still reported,
	// by DD015 as an unused image, so nothing falls through the gap.
	m.Dangling = len(m.RepoTags) == 0 && len(m.RepoDigests) == 0

	// Architecture, OS and layer count are only available from an inspect.
	// A failure here costs three optional fields, not the scan.
	if insp, err := c.api.ImageInspect(ctx, s.ID); err == nil {
		m.Architecture = insp.Architecture
		m.OS = insp.Os
		m.Layers = len(insp.RootFS.Layers)
	}

	return m
}

// cleanRefs drops Docker's "<none>:<none>" and "<none>@<none>" placeholders,
// which mean "no reference" rather than being real references.
func cleanRefs(refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if r == "" || r == "<none>:<none>" || r == "<none>@<none>" {
			continue
		}
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}
