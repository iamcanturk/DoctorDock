package rules

import (
	"context"
	"fmt"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// --- DD017 ------------------------------------------------------------------

// UnusedVolume reports volumes that no container mounts.
type UnusedVolume struct{}

func (UnusedVolume) ID() string               { return "DD017" }
func (UnusedVolume) Name() string             { return "Unused volume" }
func (UnusedVolume) Category() model.Category { return model.CategoryCleanup }
func (UnusedVolume) Severity() model.Severity { return model.SeverityInfo }
func (UnusedVolume) Description() string {
	return "Reports volumes that no container mounts. These are reported only — DoctorDock never " +
		"deletes a volume, because an unused volume can still hold the only copy of real data."
}

func (r UnusedVolume) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, v := range t.Environment.Volumes {
		if v.InUse {
			continue
		}
		f := newVolumeFinding(r, v)

		if v.IsAnonymous() {
			f.Title = fmt.Sprintf("Unused anonymous volume %s", shorten(v.Name, 12))
			f.Description = "Nothing mounts this volume, and nothing names it either — Docker " +
				"created it implicitly for a container that has since been removed. Anonymous " +
				"volumes accumulate quietly and are the usual answer to \"where did my disk go\"."
		} else {
			f.Title = fmt.Sprintf("Unused volume %s", v.Name)
			f.Description = "No container currently mounts this volume. That may be correct — a " +
				"stopped project's data volume looks exactly the same as an abandoned one."
		}

		f.Recommendation = fmt.Sprintf(
			"Check the contents at %s before doing anything. If it is genuinely disposable, "+
				"remove it with `docker volume rm %s`. DoctorDock will not delete it for you.",
			v.Mountpoint, v.Name)
		f.Details = map[string]string{
			"driver":     v.Driver,
			"mountpoint": v.Mountpoint,
			"anonymous":  fmt.Sprintf("%t", v.IsAnonymous()),
		}
		out = append(out, f)
	}
	return out
}

// --- DD018 ------------------------------------------------------------------

// UnusedNetwork reports user-defined networks with nothing attached.
type UnusedNetwork struct{}

func (UnusedNetwork) ID() string               { return "DD018" }
func (UnusedNetwork) Name() string             { return "Unused network" }
func (UnusedNetwork) Category() model.Category { return model.CategoryCleanup }
func (UnusedNetwork) Severity() model.Severity { return model.SeverityInfo }
func (UnusedNetwork) Description() string {
	return "Reports user-defined networks with no attached containers. Docker's predefined networks " +
		"are never reported."
}

func (r UnusedNetwork) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, n := range t.Environment.Networks {
		// bridge, host, none and ingress always exist and cannot be removed.
		if n.IsBuiltin() || n.InUse() {
			continue
		}
		f := newNetworkFinding(r, n)
		f.Title = fmt.Sprintf("Unused network %s", n.Name)
		f.Description = "No container is attached to this network. Compose leaves these behind when " +
			"a project is brought down with `docker compose down` without `--volumes`, and each one " +
			"still consumes an address range from the pool Docker allocates bridge networks from."
		f.Recommendation = fmt.Sprintf(
			"Remove it with `docker network rm %s`, or clear every unused network with "+
				"`docker network prune`.", n.Name)
		f.Details = map[string]string{"driver": n.Driver, "scope": n.Scope}
		out = append(out, f)
	}
	return out
}

func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
