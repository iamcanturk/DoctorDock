//go:build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// These tests exercise the real removal calls against a real daemon, on
// resources they create themselves and address by exact ID.
//
// The e2e script cannot cover this: `doctordock cleanup --apply` operates on
// the whole environment, so running it on a developer's machine would delete
// their resources. Here every removal names one identifier that this test just
// created, so nothing else can be affected.

func pruner(t *testing.T) (docker.Pruner, context.Context) {
	t.Helper()

	client, ctx := connect(t)
	p, ok := docker.AsPruner(client)
	if !ok {
		t.Fatal("the production client should implement Pruner")
	}
	return p, ctx
}

// dockerCLI runs a docker command for fixture setup. The production code never
// shells out; a test creating its own fixtures is a different matter, and using
// the CLI here keeps the setup independent of the code under test.
func dockerCLI(t *testing.T, args ...string) string {
	t.Helper()

	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func exists(args ...string) bool {
	return exec.Command("docker", args...).Run() == nil
}

func TestRemoveNetwork(t *testing.T) {
	p, ctx := pruner(t)

	const name = "ddtest-prune-network"
	dockerCLI(t, "network", "rm", "-f", name)
	id := dockerCLI(t, "network", "create", name)
	t.Cleanup(func() { _ = exec.Command("docker", "network", "rm", "-f", name).Run() })

	if err := p.RemoveNetwork(ctx, id); err != nil {
		t.Fatalf("RemoveNetwork: %v", err)
	}
	if exists("network", "inspect", name) {
		t.Error("the network still exists after removal")
	}
}

func TestRemoveVolume(t *testing.T) {
	p, ctx := pruner(t)

	const name = "ddtest-prune-volume"
	dockerCLI(t, "volume", "create", name)
	t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", "-f", name).Run() })

	if err := p.RemoveVolume(ctx, name); err != nil {
		t.Fatalf("RemoveVolume: %v", err)
	}
	if exists("volume", "inspect", name) {
		t.Error("the volume still exists after removal")
	}
}

func TestRemoveContainerDoesNotTakeItsVolume(t *testing.T) {
	p, ctx := pruner(t)

	const (
		container = "ddtest-prune-container"
		volume    = "ddtest-prune-kept-volume"
	)
	dockerCLI(t, "volume", "create", volume)
	dockerCLI(t, "run", "--name", container, "-v", volume+":/data", "alpine:3.20", "true")

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", container).Run()
		_ = exec.Command("docker", "volume", "rm", "-f", volume).Run()
	})

	id := dockerCLI(t, "inspect", "--format", "{{.Id}}", container)

	if err := p.RemoveContainer(ctx, id); err != nil {
		t.Fatalf("RemoveContainer: %v", err)
	}
	if exists("inspect", container) {
		t.Error("the container still exists after removal")
	}

	// The guarantee that matters: removing a container must never take its
	// volumes with it, or the explicit --volumes gate would be worthless.
	if !exists("volume", "inspect", volume) {
		t.Fatal("removing a container destroyed its volume — the --volumes gate was bypassed")
	}
}

func TestRemoveContainerRefusesToKillARunningOne(t *testing.T) {
	p, ctx := pruner(t)

	const name = "ddtest-prune-running"
	dockerCLI(t, "run", "-d", "--name", name, "alpine:3.20", "sleep", "60")
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	id := dockerCLI(t, "inspect", "--format", "{{.Id}}", name)

	// Force is deliberately off. A container that started running between the
	// scan and the apply must survive, and the daemon's refusal is the
	// mechanism that guarantees it.
	if err := p.RemoveContainer(ctx, id); err == nil {
		t.Fatal("removing a running container should be refused, not forced")
	}
	if !exists("inspect", name) {
		t.Fatal("the running container was removed anyway")
	}
}

func TestRemoveImage(t *testing.T) {
	p, ctx := pruner(t)

	const tag = "ddtest-prune-image:v1"
	dir := t.TempDir()
	if err := writeFile(dir+"/Dockerfile", "FROM alpine:3.20\nRUN echo prune > /marker\n"); err != nil {
		t.Fatal(err)
	}
	dockerCLI(t, "build", "-q", "-t", tag, dir)
	t.Cleanup(func() { _ = exec.Command("docker", "rmi", "-f", tag).Run() })

	id := dockerCLI(t, "inspect", "--format", "{{.Id}}", tag)

	if err := p.RemoveImage(ctx, model.Image{ID: id, RepoTags: []string{tag}}); err != nil {
		t.Fatalf("RemoveImage: %v", err)
	}
	if exists("image", "inspect", tag) {
		t.Error("the image still exists after removal")
	}
}

// TestRemoveImageWithSeveralTags covers the case that would otherwise need
// Force: the daemon refuses to remove a multi-tag image by ID, so the pruner
// removes each tag instead.
func TestRemoveImageWithSeveralTags(t *testing.T) {
	p, ctx := pruner(t)

	const (
		tagA = "ddtest-prune-multi:a"
		tagB = "ddtest-prune-multi:b"
	)
	dir := t.TempDir()
	if err := writeFile(dir+"/Dockerfile", "FROM alpine:3.20\nRUN echo multi > /marker\n"); err != nil {
		t.Fatal(err)
	}
	dockerCLI(t, "build", "-q", "-t", tagA, dir)
	dockerCLI(t, "tag", tagA, tagB)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", tagA).Run()
		_ = exec.Command("docker", "rmi", "-f", tagB).Run()
	})

	id := dockerCLI(t, "inspect", "--format", "{{.Id}}", tagA)

	err := p.RemoveImage(ctx, model.Image{ID: id, RepoTags: []string{tagA, tagB}})
	if err != nil {
		t.Fatalf("RemoveImage with two tags: %v", err)
	}
	if exists("image", "inspect", tagA) || exists("image", "inspect", tagB) {
		t.Error("a tag survived removal")
	}
}

// TestRemoveImageRefusesWhenInUse verifies that Force stays off: an image a
// container depends on must not disappear from under it.
func TestRemoveImageRefusesWhenInUse(t *testing.T) {
	p, ctx := pruner(t)

	const (
		tag       = "ddtest-prune-inuse:v1"
		container = "ddtest-prune-inuse-container"
	)
	dir := t.TempDir()
	if err := writeFile(dir+"/Dockerfile", "FROM alpine:3.20\nRUN echo inuse > /marker\n"); err != nil {
		t.Fatal(err)
	}
	dockerCLI(t, "build", "-q", "-t", tag, dir)
	dockerCLI(t, "run", "-d", "--name", container, tag, "sleep", "60")
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", container).Run()
		_ = exec.Command("docker", "rmi", "-f", tag).Run()
	})

	id := dockerCLI(t, "inspect", "--format", "{{.Id}}", tag)

	if err := p.RemoveImage(ctx, model.Image{ID: id, RepoTags: []string{tag}}); err == nil {
		t.Fatal("removing an in-use image should be refused, not forced")
	}
	if !exists("image", "inspect", tag) {
		t.Fatal("the in-use image was removed anyway")
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
