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
//
// SharedSize is deliberately NOT requested. Computing it makes the daemon walk
// the whole layer graph, which on a machine with dozens of images takes several
// seconds of VM CPU — the single largest cost in a scan, and the reason it felt
// like it was straining the machine. Nothing reads SharedSize, so asking for it
// was pure waste. Size (per-image total) needs no graph walk and is kept.
func (c *engineClient) ListImages(ctx context.Context) ([]model.Image, error) {
	summaries, err := c.api.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	// buildImage is pure now — no per-image inspect — so there is nothing to
	// parallelise. Everything comes from the list response.
	out := make([]model.Image, len(summaries))
	for i := range summaries {
		out[i] = buildImage(summaries[i])
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].DisplayName(), out[j].DisplayName()
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// buildImage normalizes an image from the list response alone.
//
// It used to inspect every image to fill in Architecture, OS and layer count.
// That was one API round-trip per image — half a second of daemon work on a
// machine with a few dozen images, repeated on every background refresh — for
// three fields nothing displays and no rule reads. Dropping it makes a scan
// several times faster and takes real load off the daemon. If a future rule
// needs the architecture (an amd64 image on an arm64 host, say), it can inspect
// only the images it cares about rather than all of them up front.
func buildImage(s image.Summary) model.Image {
	m := model.Image{
		ID:          s.ID,
		RepoTags:    cleanRefs(s.RepoTags),
		RepoDigests: cleanRefs(s.RepoDigests),
		Size:        s.Size,
		// -1 means "not computed": see ListImages on why shared size is skipped.
		SharedSize: -1,
		Created:    time.Unix(s.Created, 0).UTC(),
		Labels:     s.Labels,
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
