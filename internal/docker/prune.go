package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Pruner removes Docker resources.
//
// It is a separate interface from Client, and Client will never gain these
// methods. The scanner is handed a Client, so a scan cannot delete anything —
// that guarantee is enforced by the type system rather than by discipline. Only
// the cleanup command asks for a Pruner. See
// docs/adr/0006-cleanup-safety-model.md.
type Pruner interface {
	// RemoveContainer removes a stopped container. It never removes the
	// container's anonymous volumes: that would route around the explicit
	// --volumes gate and destroy data the user did not approve losing.
	RemoveContainer(ctx context.Context, id string) error

	// RemoveImage removes an image. It never forces: if the daemon refuses
	// because something started using the image since the scan, that refusal
	// is the point.
	RemoveImage(ctx context.Context, img model.Image) error

	// RemoveVolume removes a volume. This is the only irreversible operation
	// DoctorDock performs.
	RemoveVolume(ctx context.Context, name string) error

	// RemoveNetwork removes a network.
	RemoveNetwork(ctx context.Context, id string) error
}

// AsPruner returns the Pruner behind a Client, if it has one. The production
// client does; a caller holding some other implementation gets false rather
// than a panic.
func AsPruner(c Client) (Pruner, bool) {
	p, ok := c.(Pruner)
	return p, ok
}

var _ Pruner = (*engineClient)(nil)

func (c *engineClient) RemoveContainer(ctx context.Context, id string) error {
	err := c.api.ContainerRemove(ctx, id, container.RemoveOptions{
		// Both deliberately false. RemoveVolumes would take anonymous volumes
		// with the container, bypassing the --volumes gate entirely; Force
		// would kill a container that started running since the scan.
		RemoveVolumes: false,
		Force:         false,
	})
	if err != nil {
		return fmt.Errorf("remove container %s: %w", model.ShortID(id), err)
	}
	return nil
}

func (c *engineClient) RemoveImage(ctx context.Context, img model.Image) error {
	// An image with several tags cannot be removed by ID without forcing —
	// the daemon refuses with "image is referenced in multiple repositories".
	// Removing each tag instead achieves the same result without Force, and
	// the daemon drops the image once the last reference goes.
	refs := img.RepoTags
	if len(refs) == 0 {
		refs = []string{img.ID}
	}

	for _, ref := range refs {
		_, err := c.api.ImageRemove(ctx, ref, image.RemoveOptions{
			Force: false,
			// Untagged parent layers left behind by this image are garbage the
			// moment it goes, and removing them is what reclaims the space the
			// plan promised.
			PruneChildren: true,
		})
		if err != nil {
			return fmt.Errorf("remove image %s: %w", ref, err)
		}
	}
	return nil
}

func (c *engineClient) RemoveVolume(ctx context.Context, name string) error {
	// force=false: if a container claimed the volume between the scan and now,
	// the daemon must refuse.
	if err := c.api.VolumeRemove(ctx, name, false); err != nil {
		return fmt.Errorf("remove volume %s: %w", name, err)
	}
	return nil
}

func (c *engineClient) RemoveNetwork(ctx context.Context, id string) error {
	if err := c.api.NetworkRemove(ctx, id); err != nil {
		return fmt.Errorf("remove network %s: %w", model.ShortID(id), err)
	}
	return nil
}

// --- test double -------------------------------------------------------------

// FakePruner records removals instead of performing them. It lives in the
// non-test build so that other packages' tests can use it.
type FakePruner struct {
	// Removed records every successful removal as "kind:identifier", in order.
	Removed []string

	// Errors maps an identifier to the error its removal should return, for
	// exercising the failure path.
	Errors map[string]error
}

var _ Pruner = (*FakePruner)(nil)

func (f *FakePruner) record(kind, id string) error {
	if err, ok := f.Errors[id]; ok {
		return err
	}
	f.Removed = append(f.Removed, kind+":"+id)
	return nil
}

func (f *FakePruner) RemoveContainer(_ context.Context, id string) error {
	return f.record("container", id)
}

func (f *FakePruner) RemoveImage(_ context.Context, img model.Image) error {
	return f.record("image", img.ID)
}

func (f *FakePruner) RemoveVolume(_ context.Context, name string) error {
	return f.record("volume", name)
}

func (f *FakePruner) RemoveNetwork(_ context.Context, id string) error {
	return f.record("network", id)
}
