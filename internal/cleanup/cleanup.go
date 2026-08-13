// Package cleanup decides what can be removed from a Docker environment, and
// removes it when asked.
//
// Planning and applying are separate functions on purpose: the plan is what a
// dry run prints, what the confirmation prompt describes, and what the user
// approves. Apply only ever executes a plan that already exists.
//
// See docs/adr/0006-cleanup-safety-model.md.
package cleanup

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Targets selects what a cleanup considers.
//
// Volumes is deliberately not covered by any "everything" convenience: see
// Targets.All, which leaves it alone.
type Targets struct {
	// Containers covers stopped containers.
	Containers bool
	// Images covers unused tagged images. It implies DanglingImages.
	Images bool
	// DanglingImages covers untagged leftovers.
	DanglingImages bool
	// Networks covers user-defined networks with nothing attached.
	Networks bool
	// Volumes covers volumes no container mounts. This is the only target that
	// can destroy data, and nothing enables it implicitly.
	Volumes bool
}

// DefaultTargets is what a cleanup with no target flags considers: only the
// things Docker itself calls safe to prune.
func DefaultTargets() Targets {
	return Targets{DanglingImages: true, Networks: true}
}

// All covers everything except volumes.
//
// Every resource here can be recreated — an image pulled or rebuilt, a
// container recreated, a network re-declared. A volume's contents cannot, which
// is why it is absent.
func All() Targets {
	return Targets{Containers: true, Images: true, DanglingImages: true, Networks: true}
}

// Any reports whether at least one target is selected.
func (t Targets) Any() bool {
	return t.Containers || t.Images || t.DanglingImages || t.Networks || t.Volumes
}

// Options controls planning.
type Options struct {
	Targets Targets

	// KeepSince protects resources created within this window. Zero protects
	// nothing. `--keep-since 24h` will not remove an image built this morning.
	KeepSince time.Duration

	// Now overrides the clock, for tests.
	Now time.Time
}

func (o Options) now() time.Time {
	if !o.Now.IsZero() {
		return o.Now
	}
	return time.Now()
}

// tooRecent reports whether a resource is inside the KeepSince window.
func (o Options) tooRecent(created time.Time) bool {
	if o.KeepSince <= 0 || created.IsZero() {
		return false
	}
	return o.now().Sub(created) < o.KeepSince
}

// Plan works out what would be removed, without touching anything.
//
// Ordering is accounted for: an image referenced only by a stopped container
// that is itself being removed counts as unused, because it will be by the time
// the images are reached. Without that, the user would have to run cleanup
// twice for it to converge.
func Plan(env *model.Environment, opts Options) []model.CleanupItem {
	var items []model.CleanupItem

	doomed := doomedContainers(env, opts)

	items = append(items, planContainers(env, opts, doomed)...)
	items = append(items, planImages(env, opts, doomed)...)
	items = append(items, planNetworks(env, opts, doomed)...)
	items = append(items, planVolumes(env, opts, doomed)...)

	return items
}

// doomedContainers returns the names of containers this plan will remove, so
// that usage of other resources can be recomputed as it will be afterwards.
func doomedContainers(env *model.Environment, opts Options) map[string]bool {
	doomed := make(map[string]bool)
	if !opts.Targets.Containers {
		return doomed
	}
	for _, c := range env.Containers {
		if removableContainer(c, opts) {
			doomed[c.Name] = true
		}
	}
	return doomed
}

// removableContainer reports whether a container is stopped and outside the
// protection window.
func removableContainer(c model.Container, opts Options) bool {
	switch c.State {
	case model.StateExited, model.StateCreated, model.StateDead:
	default:
		// Running, restarting, paused and removing are all left alone.
		return false
	}
	return !opts.tooRecent(c.Created)
}

func planContainers(env *model.Environment, opts Options, _ map[string]bool) []model.CleanupItem {
	if !opts.Targets.Containers {
		return nil
	}

	var items []model.CleanupItem
	for _, c := range env.Containers {
		if !removableContainer(c, opts) {
			continue
		}
		items = append(items, model.CleanupItem{
			Resource: model.ResourceContainer,
			ID:       c.ID,
			Name:     c.Name,
			Reason:   fmt.Sprintf("stopped (%s)", orUnknown(c.Status)),
			// A stopped container is often stopped on purpose, and removing it
			// discards its logs and its writable layer.
			Risk: model.RiskReview,
			Size: -1,
		})
	}
	return items
}

func planImages(env *model.Environment, opts Options, doomed map[string]bool) []model.CleanupItem {
	if !opts.Targets.Images && !opts.Targets.DanglingImages {
		return nil
	}

	var items []model.CleanupItem
	for _, img := range env.Images {
		if usedAfterCleanup(img.UsedBy, img.InUse, doomed) {
			continue
		}
		if opts.tooRecent(img.Created) {
			continue
		}

		switch {
		case img.Dangling:
			if !opts.Targets.DanglingImages && !opts.Targets.Images {
				continue
			}
			items = append(items, model.CleanupItem{
				Resource: model.ResourceImage,
				ID:       img.ID,
				Name:     img.DisplayName(),
				Reason:   "dangling: no tag, no digest, nothing references it",
				Risk:     model.RiskSafe,
				Size:     img.Size,
			})
		default:
			if !opts.Targets.Images {
				continue
			}
			items = append(items, model.CleanupItem{
				Resource: model.ResourceImage,
				ID:       img.ID,
				Name:     img.DisplayName(),
				Reason:   reasonUnused(img.UsedBy, doomed, "no container references it"),
				// A tagged image may be a base you pull constantly; removing it
				// costs a download, not data.
				Risk: model.RiskReview,
				Size: img.Size,
			})
		}
	}
	return items
}

func planNetworks(env *model.Environment, opts Options, doomed map[string]bool) []model.CleanupItem {
	if !opts.Targets.Networks {
		return nil
	}

	var items []model.CleanupItem
	for _, n := range env.Networks {
		// bridge, host, none and ingress always exist and cannot be removed.
		if n.IsBuiltin() {
			continue
		}
		if usedAfterCleanup(n.Containers, n.InUse(), doomed) {
			continue
		}
		if opts.tooRecent(n.Created) {
			continue
		}
		items = append(items, model.CleanupItem{
			Resource: model.ResourceNetwork,
			ID:       n.ID,
			Name:     n.Name,
			Reason:   reasonUnused(n.Containers, doomed, "no container is attached"),
			Risk:     model.RiskSafe,
			Size:     -1,
		})
	}
	return items
}

func planVolumes(env *model.Environment, opts Options, doomed map[string]bool) []model.CleanupItem {
	if !opts.Targets.Volumes {
		return nil
	}

	var items []model.CleanupItem
	for _, v := range env.Volumes {
		if usedAfterCleanup(v.UsedBy, v.InUse, doomed) {
			continue
		}
		if opts.tooRecent(v.Created) {
			continue
		}

		reason := reasonUnused(v.UsedBy, doomed, "no container mounts it")
		if v.IsAnonymous() {
			reason += "; anonymous, created implicitly for a container that is gone"
		}

		items = append(items, model.CleanupItem{
			Resource: model.ResourceVolume,
			ID:       v.Name,
			Name:     v.Name,
			Reason:   reason,
			// The only irreversible one. An abandoned volume and the only copy
			// of a database look identical from the API.
			Risk: model.RiskDataLoss,
			Size: v.Size,
		})
	}
	return items
}

// usedAfterCleanup reports whether a resource will still be referenced once the
// doomed containers are gone.
func usedAfterCleanup(usedBy []string, inUse bool, doomed map[string]bool) bool {
	if !inUse {
		return false
	}
	// InUse is true but the referencing containers are unknown; treat it as
	// used rather than guess.
	if len(usedBy) == 0 {
		return true
	}
	for _, name := range usedBy {
		if !doomed[name] {
			return true
		}
	}
	return false
}

func reasonUnused(usedBy []string, doomed map[string]bool, plain string) string {
	var freed []string
	for _, name := range usedBy {
		if doomed[name] {
			freed = append(freed, name)
		}
	}
	if len(freed) == 0 {
		return plain
	}
	sort.Strings(freed)
	return fmt.Sprintf("only used by %s, which this cleanup also removes", joinNames(freed))
}

func joinNames(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return fmt.Sprintf("%s and %d others", names[0], len(names)-1)
	}
}

func orUnknown(s string) string {
	if s == "" {
		return "state unknown"
	}
	return s
}

// Apply executes a plan and returns it annotated with what happened.
//
// Order matters: containers first, because an image or volume they hold cannot
// be removed until they are gone. Volumes go last, so that a failure earlier in
// the plan stops before the irreversible part.
//
// A failure on one item never aborts the rest — a locked volume should not
// prevent a dangling image from being reclaimed — but every failure is recorded
// on the item and counted in the summary.
func Apply(ctx context.Context, p docker.Pruner, items []model.CleanupItem) []model.CleanupItem {
	out := make([]model.CleanupItem, len(items))
	copy(out, items)

	order := []model.ResourceKind{
		model.ResourceContainer,
		model.ResourceImage,
		model.ResourceNetwork,
		model.ResourceVolume,
	}

	for _, kind := range order {
		for i := range out {
			if out[i].Resource != kind {
				continue
			}
			if err := ctx.Err(); err != nil {
				out[i].Error = err.Error()
				continue
			}
			if err := remove(ctx, p, out[i]); err != nil {
				out[i].Error = err.Error()
				continue
			}
			out[i].Removed = true
		}
	}

	return out
}

func remove(ctx context.Context, p docker.Pruner, item model.CleanupItem) error {
	switch item.Resource {
	case model.ResourceContainer:
		return p.RemoveContainer(ctx, item.ID)
	case model.ResourceImage:
		// RemoveImage needs the tags to avoid forcing on a multi-tag image;
		// the plan carries the display name, so reconstruct the minimum the
		// pruner needs.
		return p.RemoveImage(ctx, imageRef(item))
	case model.ResourceNetwork:
		return p.RemoveNetwork(ctx, item.ID)
	case model.ResourceVolume:
		return p.RemoveVolume(ctx, item.ID)
	default:
		return fmt.Errorf("cannot remove resource of kind %q", item.Resource)
	}
}

// imageRef rebuilds the minimal model.Image the pruner needs. A dangling image
// has no usable name, so it is removed by ID.
func imageRef(item model.CleanupItem) model.Image {
	img := model.Image{ID: item.ID}
	shortID := img.ShortID()
	if item.Name != "" && item.Name != "<none>:<none>" && item.Name != shortID {
		img.RepoTags = []string{item.Name}
	}
	return img
}

// NewPlan assembles the document a renderer or a client consumes.
func NewPlan(tool model.ToolInfo, items []model.CleanupItem, applied bool, now time.Time) *model.CleanupPlan {
	if items == nil {
		items = []model.CleanupItem{}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &model.CleanupPlan{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   now,
		Tool:          tool,
		Applied:       applied,
		Items:         items,
		Summary:       model.SummarizeCleanup(items),
	}
}
