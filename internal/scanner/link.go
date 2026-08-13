package scanner

import (
	"sort"
	"strings"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// link resolves the relationships between resources: which images are
// referenced, which volumes are mounted, which networks have containers.
//
// The daemon does not give this to us reliably. The image list's Containers
// count is documented as "not calculated by default", the volume list carries
// no back-reference at all, and the network list endpoint omits the container
// map. All three are derivable from the container list, so they are derived
// once here rather than by every rule that needs them. See
// docs/adr/0003-whole-environment-rule-target.md.
func link(env *model.Environment) {
	linkImages(env)
	linkVolumes(env)
	linkNetworks(env)
}

func linkImages(env *model.Environment) {
	// A container can name its image two ways that both have to resolve: by
	// the ID it was actually started from, and by the reference the user wrote.
	// The two diverge as soon as a tag is rebuilt — the container keeps
	// pointing at the old ID while the tag names a new image. Matching both
	// means neither the running image nor the freshly built one is reported as
	// unused.
	byID := make(map[string]int, len(env.Images))
	byRef := make(map[string][]int, len(env.Images))

	for i, img := range env.Images {
		byID[img.ID] = i
		byID[strings.TrimPrefix(img.ID, "sha256:")] = i
		for _, ref := range img.RepoTags {
			byRef[ref] = append(byRef[ref], i)
			// `docker run nginx` records the image as "nginx", which the
			// daemon stores tagged as "nginx:latest".
			if model.TagOf(ref) == "latest" {
				byRef[model.RepositoryOf(ref)] = append(byRef[model.RepositoryOf(ref)], i)
			}
		}
		for _, ref := range img.RepoDigests {
			byRef[ref] = append(byRef[ref], i)
		}
	}

	markUsed := func(idx int, containerName string) {
		env.Images[idx].InUse = true
		env.Images[idx].UsedBy = appendUnique(env.Images[idx].UsedBy, containerName)
	}

	for _, c := range env.Containers {
		if idx, ok := byID[c.ImageID]; ok {
			markUsed(idx, c.Name)
		}
		for _, idx := range byRef[c.Image] {
			markUsed(idx, c.Name)
		}
	}

	for i := range env.Images {
		sort.Strings(env.Images[i].UsedBy)
	}
}

func linkVolumes(env *model.Environment) {
	byName := make(map[string]int, len(env.Volumes))
	for i, v := range env.Volumes {
		byName[v.Name] = i
	}

	for _, c := range env.Containers {
		for _, m := range c.Mounts {
			// Bind mounts have no volume record; only named and anonymous
			// volumes appear in the volume list.
			name := m.Name
			if name == "" {
				continue
			}
			idx, ok := byName[name]
			if !ok {
				continue
			}
			env.Volumes[idx].InUse = true
			env.Volumes[idx].UsedBy = appendUnique(env.Volumes[idx].UsedBy, c.Name)
		}
	}

	for i := range env.Volumes {
		sort.Strings(env.Volumes[i].UsedBy)
	}
}

func linkNetworks(env *model.Environment) {
	byName := make(map[string]int, len(env.Networks))
	for i, n := range env.Networks {
		byName[n.Name] = i
	}

	for _, c := range env.Containers {
		for _, netName := range c.Networks {
			idx, ok := byName[netName]
			if !ok {
				continue
			}
			env.Networks[idx].Containers = appendUnique(env.Networks[idx].Containers, c.Name)
		}
	}

	for i := range env.Networks {
		sort.Strings(env.Networks[i].Containers)
	}
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}
