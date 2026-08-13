package docker

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// ListContainers returns every container on the daemon, running or not.
//
// The list endpoint alone is not enough: privileged mode, capabilities,
// mounts, the configured user and resource limits only appear in an inspect.
// Inspects run with bounded concurrency.
func (c *engineClient) ListContainers(ctx context.Context) ([]model.Container, error) {
	summaries, err := c.api.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	out := make([]model.Container, len(summaries))
	forEach(len(summaries), func(i int) {
		out[i] = c.buildContainer(ctx, summaries[i])
	})

	// The daemon returns newest-first. Sorting by name makes report output
	// stable across runs, which matters for diffing two scans.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// buildContainer normalizes one container. An inspect failure is not fatal:
// containers can disappear between the list and the inspect, and a partial
// record is more useful than aborting the whole scan.
func (c *engineClient) buildContainer(ctx context.Context, s container.Summary) model.Container {
	m := model.Container{
		ID:       s.ID,
		Name:     containerName(s),
		Image:    s.Image,
		ImageID:  s.ImageID,
		State:    string(s.State),
		Status:   s.Status,
		Created:  time.Unix(s.Created, 0).UTC(),
		Ports:    convertPorts(s.Ports),
		Health:   model.HealthNone,
		Labels:   s.Labels,
		Networks: []string{},
		Mounts:   []model.Mount{},
		EnvKeys:  []string{},
		CapAdd:   []string{},
		CapDrop:  []string{},
	}
	if m.Ports == nil {
		m.Ports = []model.Port{}
	}

	insp, err := c.api.ContainerInspect(ctx, s.ID)
	if err != nil {
		return m
	}

	if insp.Config != nil {
		m.User = insp.Config.User
		m.EnvKeys = envKeys(insp.Config.Env)
		m.HasHealthcheck = hasHealthcheck(insp.Config.Healthcheck)
		if insp.Config.Labels != nil {
			m.Labels = insp.Config.Labels
		}
	}

	if insp.ContainerJSONBase != nil {
		m.RestartCount = insp.RestartCount
		if insp.Name != "" {
			m.Name = strings.TrimPrefix(insp.Name, "/")
		}
		if insp.State != nil {
			m.StartedAt = parseDockerTime(insp.State.StartedAt)
			if insp.State.Health != nil {
				m.Health = strings.ToLower(string(insp.State.Health.Status))
				// A live health status proves a healthcheck exists even if the
				// config was inherited in a form we did not recognise.
				m.HasHealthcheck = true
			}
		}
		if hc := insp.HostConfig; hc != nil {
			m.Privileged = hc.Privileged
			m.NetworkMode = string(hc.NetworkMode)
			m.PidMode = string(hc.PidMode)
			m.IpcMode = string(hc.IpcMode)
			m.CapAdd = normalizeCaps(hc.CapAdd)
			m.CapDrop = normalizeCaps(hc.CapDrop)
			m.ReadOnlyRootFS = hc.ReadonlyRootfs
			m.MemoryLimit = hc.Memory
			m.NanoCPUs = hc.NanoCPUs
			if hc.PidsLimit != nil {
				m.PidsLimit = *hc.PidsLimit
			}
			m.RestartPolicy = string(hc.RestartPolicy.Name)
		}
	}

	m.Mounts = convertMounts(insp.Mounts)
	if insp.NetworkSettings != nil {
		m.Networks = sortedKeys(insp.NetworkSettings.Networks)
	}

	m.EffectiveUser = c.resolveEffectiveUser(ctx, m.User, s.ImageID)

	return m
}

// resolveEffectiveUser determines the user the container actually runs as.
//
// An empty container-level user means the image's USER directive applies, and
// an image with no USER runs as root. Without this resolution DD001 would
// either miss every container that inherits root from its image, or flag every
// container that correctly sets USER in its Dockerfile.
func (c *engineClient) resolveEffectiveUser(ctx context.Context, configured, imageID string) string {
	if u := strings.TrimSpace(configured); u != "" {
		return u
	}
	if imageID == "" {
		return "root"
	}

	c.imageUserMu.Lock()
	cached, ok := c.imageUserCache[imageID]
	c.imageUserMu.Unlock()
	if ok {
		return cached
	}

	user := "root"
	if insp, err := c.api.ImageInspect(ctx, imageID); err == nil && insp.Config != nil {
		if u := strings.TrimSpace(insp.Config.User); u != "" {
			user = u
		}
	}

	c.imageUserMu.Lock()
	c.imageUserCache[imageID] = user
	c.imageUserMu.Unlock()
	return user
}

func containerName(s container.Summary) string {
	if len(s.Names) == 0 {
		return model.Container{ID: s.ID}.ShortID()
	}
	return strings.TrimPrefix(s.Names[0], "/")
}

// envKeys extracts variable names from KEY=VALUE strings and discards every
// value. This is the single choke point that makes leaking a secret into a
// report impossible. See docs/adr/0005-no-secret-collection.md.
func envKeys(env []string) []string {
	keys := make([]string, 0, len(env))
	for _, e := range env {
		if key, _, found := strings.Cut(e, "="); found {
			keys = append(keys, key)
		} else if e != "" {
			keys = append(keys, e)
		}
	}
	sort.Strings(keys)
	return keys
}

// hasHealthcheck reports whether the container has a usable healthcheck.
// A test of exactly ["NONE"] is Docker's way of disabling one inherited from
// the image, so it counts as absent.
func hasHealthcheck(hc *container.HealthConfig) bool {
	if hc == nil || len(hc.Test) == 0 {
		return false
	}
	return !strings.EqualFold(hc.Test[0], "NONE")
}

type portKey struct {
	private, public uint16
	proto           string
	wildcard        bool
}

// convertPorts normalizes published ports and collapses dual-stack duplicates.
//
// A single `-p 8080:80` on a dual-stack host is reported by the daemon as two
// entries, one for 0.0.0.0 and one for ::. They describe one binding, so
// keeping both would double every port in the report and make DD006 fire twice
// for the same exposure.
func convertPorts(ports []container.Port) []model.Port {
	out := make([]model.Port, 0, len(ports))
	index := make(map[portKey]int, len(ports))

	for _, p := range ports {
		mp := model.Port{
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Type:        p.Type,
			HostIP:      p.IP,
		}
		key := portKey{p.PrivatePort, p.PublicPort, p.Type, mp.IsPublicallyBound()}

		if at, ok := index[key]; ok {
			// Prefer the IPv4 spelling; it is the one users recognise from
			// `docker ps`.
			if strings.Contains(out[at].HostIP, ":") && !strings.Contains(mp.HostIP, ":") {
				out[at].HostIP = mp.HostIP
			}
			continue
		}
		index[key] = len(out)
		out = append(out, mp)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].PrivatePort != out[j].PrivatePort {
			return out[i].PrivatePort < out[j].PrivatePort
		}
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].HostIP < out[j].HostIP
	})
	return out
}

func convertMounts(mounts []container.MountPoint) []model.Mount {
	out := make([]model.Mount, 0, len(mounts))
	for _, m := range mounts {
		out = append(out, model.Mount{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
			Name:        m.Name,
			ReadOnly:    !m.RW,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Destination < out[j].Destination })
	return out
}

// normalizeCaps upper-cases capability names and strips the optional CAP_
// prefix, so that rules can compare against a single spelling. Docker accepts
// "SYS_ADMIN", "sys_admin" and "CAP_SYS_ADMIN" interchangeably.
func normalizeCaps(caps []string) []string {
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		c = strings.ToUpper(strings.TrimSpace(c))
		c = strings.TrimPrefix(c, "CAP_")
		if c != "" {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parseDockerTime parses the RFC3339Nano strings the daemon returns. Docker
// uses the zero time to mean "never", which becomes a zero time.Time here.
func parseDockerTime(s string) time.Time {
	if s == "" || strings.HasPrefix(s, "0001-01-01") {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
