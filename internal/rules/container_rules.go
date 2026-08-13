package rules

import (
	"context"
	"fmt"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// --- DD007 ------------------------------------------------------------------

// MissingHealthcheck reports containers with no healthcheck defined.
type MissingHealthcheck struct{}

func (MissingHealthcheck) ID() string               { return "DD007" }
func (MissingHealthcheck) Name() string             { return "No healthcheck" }
func (MissingHealthcheck) Category() model.Category { return model.CategoryConfiguration }
func (MissingHealthcheck) Severity() model.Severity { return model.SeverityLow }
func (MissingHealthcheck) Description() string {
	return "Reports containers with no HEALTHCHECK, so Docker cannot tell a hung process from a " +
		"healthy one."
}

func (r MissingHealthcheck) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		if c.HasHealthcheck {
			continue
		}
		f := newContainerFinding(r, c)
		f.Title = "Container has no healthcheck"
		f.Description = "Without a healthcheck, Docker only knows whether the main process is " +
			"alive, not whether it is working. A deadlocked application or one that has lost its " +
			"database connection stays \"Up\" indefinitely, and dependent services get no signal " +
			"to wait or restart."
		f.Recommendation = "Add a HEALTHCHECK to the Dockerfile, or a `healthcheck:` block in " +
			"compose. For an HTTP service, a request against a lightweight readiness endpoint is " +
			"usually enough."
		out = append(out, f)
	}
	return out
}

// --- DD008 ------------------------------------------------------------------

// MissingRestartPolicy reports containers that will not come back after a
// daemon restart or a crash.
type MissingRestartPolicy struct{}

func (MissingRestartPolicy) ID() string               { return "DD008" }
func (MissingRestartPolicy) Name() string             { return "No restart policy" }
func (MissingRestartPolicy) Category() model.Category { return model.CategoryConfiguration }
func (MissingRestartPolicy) Severity() model.Severity { return model.SeverityInfo }
func (MissingRestartPolicy) Description() string {
	return "Reports containers with no restart policy, which stay down after a crash or a host reboot."
}

func (r MissingRestartPolicy) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		if c.RestartPolicy != "" && c.RestartPolicy != "no" {
			continue
		}
		f := newContainerFinding(r, c)
		f.Title = "Container has no restart policy"
		f.Description = "The container will not be restarted after it crashes, after the Docker " +
			"daemon restarts, or after the host reboots. For a one-off task that is correct; for a " +
			"long-running service it means silent downtime."
		f.Recommendation = "Set `--restart unless-stopped` (compose: `restart: unless-stopped`) for " +
			"services that should survive a reboot. Leave it unset for one-shot jobs."
		out = append(out, f)
	}
	return out
}

// --- DD010 ------------------------------------------------------------------

// NoMemoryLimit reports running containers with no memory cap.
type NoMemoryLimit struct{}

func (NoMemoryLimit) ID() string               { return "DD010" }
func (NoMemoryLimit) Name() string             { return "No memory limit" }
func (NoMemoryLimit) Category() model.Category { return model.CategoryResource }
func (NoMemoryLimit) Severity() model.Severity { return model.SeverityLow }
func (NoMemoryLimit) Description() string {
	return "Reports running containers with no memory limit, which can consume all host memory and " +
		"trigger the OOM killer against unrelated processes."
}

func (r NoMemoryLimit) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		// A stopped container consumes nothing, so the limit only matters for
		// containers that are actually running or trying to.
		if !c.IsRunning() && c.State != model.StateRestarting {
			continue
		}
		if c.MemoryLimit > 0 {
			continue
		}
		f := newContainerFinding(r, c)
		f.Title = "Container has no memory limit"
		f.Description = "The container can allocate all available host memory. A memory leak or an " +
			"unbounded query does not just kill this container — the kernel OOM killer picks a " +
			"victim across the whole host, which is frequently a different, healthy process."
		f.Recommendation = "Set a limit with `--memory 512m` (compose: `mem_limit`, or " +
			"`deploy.resources.limits.memory`). Pick a value above observed peak usage."
		out = append(out, f)
	}
	return out
}

// --- DD011 ------------------------------------------------------------------

// MutableImageTag reports containers running from a tag whose meaning can
// change under them.
type MutableImageTag struct{}

func (MutableImageTag) ID() string               { return "DD011" }
func (MutableImageTag) Name() string             { return "Mutable image tag" }
func (MutableImageTag) Category() model.Category { return model.CategoryConfiguration }
func (MutableImageTag) Severity() model.Severity { return model.SeverityInfo }
func (MutableImageTag) Description() string {
	return "Reports containers running from a moving tag such as :latest, which makes the deployment " +
		"non-reproducible."
}

func (r MutableImageTag) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		if !model.IsMutableTag(c.Image) {
			continue
		}
		tag := model.TagOf(c.Image)
		if tag == "" {
			tag = "latest"
		}
		f := newContainerFinding(r, c)
		f.Title = fmt.Sprintf("Container uses the mutable tag :%s", tag)
		f.Description = fmt.Sprintf(
			"%q points at whatever was pushed most recently. Two hosts pulling the same reference "+
				"can end up running different code, and a rollback has nothing to roll back to.",
			c.Image)
		f.Recommendation = fmt.Sprintf(
			"Pin to an explicit version (`%s:1.2.3`) or to a digest (`%s@sha256:...`).",
			model.RepositoryOf(c.Image), model.RepositoryOf(c.Image))
		f.Details = map[string]string{"image": c.Image, "tag": tag}
		out = append(out, f)
	}
	return out
}

// --- DD012 ------------------------------------------------------------------

// UnhealthyContainer reports containers whose healthcheck is failing.
type UnhealthyContainer struct{}

func (UnhealthyContainer) ID() string               { return "DD012" }
func (UnhealthyContainer) Name() string             { return "Container is unhealthy" }
func (UnhealthyContainer) Category() model.Category { return model.CategoryPerformance }
func (UnhealthyContainer) Severity() model.Severity { return model.SeverityMedium }
func (UnhealthyContainer) Description() string {
	return "Reports containers whose own healthcheck is currently failing."
}

func (r UnhealthyContainer) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		// A stopped container keeps the health status it had when it stopped.
		// Reporting those would flag every container that was unhealthy at some
		// point in the past as a live problem.
		if !c.IsRunning() || !c.IsUnhealthy() {
			continue
		}
		f := newContainerFinding(r, c)
		f.Title = "Container is reporting unhealthy"
		f.Description = "The container's own healthcheck is failing. It is running, so Docker will " +
			"keep routing to it and dependent services will keep calling it, but by its own " +
			"definition it is not working."
		f.Recommendation = fmt.Sprintf(
			"Inspect the failure output with `docker inspect --format '{{json .State.Health}}' %s`, "+
				"then check the container logs.", c.Name)
		f.Details = map[string]string{"status": c.Status}
		out = append(out, f)
	}
	return out
}

// --- DD013 ------------------------------------------------------------------

// RestartLoop reports containers that keep dying and being restarted.
type RestartLoop struct{}

func (RestartLoop) ID() string               { return "DD013" }
func (RestartLoop) Name() string             { return "Container restart loop" }
func (RestartLoop) Category() model.Category { return model.CategoryPerformance }
func (RestartLoop) Severity() model.Severity { return model.SeverityMedium }
func (RestartLoop) Description() string {
	return "Reports containers that have been restarted repeatedly, or that are stuck in the " +
		"restarting state."
}

func (r RestartLoop) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, c := range t.Environment.Containers {
		looping := c.RestartCount >= t.Options.RestartLoopThreshold
		stuck := c.State == model.StateRestarting
		if !looping && !stuck {
			continue
		}

		f := newContainerFinding(r, c)
		if stuck {
			f.Title = "Container is stuck restarting"
			f.Description = fmt.Sprintf(
				"The container is in the restarting state and has been restarted %d times. It is "+
					"failing on startup and the restart policy keeps bringing it back.", c.RestartCount)
		} else {
			f.Title = fmt.Sprintf("Container has restarted %d times", c.RestartCount)
			f.Description = "Repeated restarts mean the process is exiting unexpectedly. Each cycle " +
				"drops in-flight work and, for a service behind a healthcheck, produces a window " +
				"where dependents see errors."
		}
		f.Recommendation = fmt.Sprintf("Check why it exits: `docker logs --tail 100 %s`.", c.Name)
		f.Details = map[string]string{
			"restart_count":  fmt.Sprintf("%d", c.RestartCount),
			"state":          c.State,
			"restart_policy": orNone(c.RestartPolicy),
		}
		out = append(out, f)
	}
	return out
}
