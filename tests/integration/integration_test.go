//go:build integration

// Package integration exercises DoctorDock against a real Docker daemon.
//
// These tests are excluded from the default build because CI runs on three
// operating systems and requiring a live daemon on all of them is fragile.
// Run them with:
//
//	go test -tags integration ./tests/integration/...
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/internal/scanner"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func connect(t *testing.T) (docker.Client, context.Context) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	client, err := docker.Connect(ctx)
	if err != nil {
		t.Skipf("no Docker daemon available: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	return client, ctx
}

func TestConnectAndInfo(t *testing.T) {
	client, ctx := connect(t)

	info, err := client.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.ServerVersion == "" {
		t.Error("the daemon reported no server version")
	}
	if info.OSType == "" || info.Architecture == "" {
		t.Errorf("incomplete daemon info: %+v", info)
	}
}

// TestCollectionIsWellFormed checks the invariants every rule and renderer
// relies on, against whatever happens to be on the machine.
func TestCollectionIsWellFormed(t *testing.T) {
	client, ctx := connect(t)

	env, err := scanner.New(client, scanner.Config{}).Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range env.Containers {
		if c.ID == "" || c.Name == "" {
			t.Errorf("container with an empty ID or name: %+v", c)
		}
		if c.State == "" {
			t.Errorf("container %s has no state", c.Name)
		}
		if c.EffectiveUser == "" {
			t.Errorf("container %s has no resolved effective user", c.Name)
		}
		// The privacy guarantee, verified against real containers rather than
		// fixtures: no collected value may look like a KEY=VALUE pair.
		for _, key := range c.EnvKeys {
			for _, r := range key {
				if r == '=' {
					t.Fatalf("container %s: env key %q contains a value", c.Name, key)
				}
			}
		}
	}

	for _, img := range env.Images {
		if img.ID == "" {
			t.Error("image with an empty ID")
		}
		if img.Size < 0 {
			t.Errorf("image %s has a negative size", img.ShortID())
		}
	}

	for _, v := range env.Volumes {
		if v.Name == "" {
			t.Error("volume with an empty name")
		}
	}

	for _, n := range env.Networks {
		if n.ID == "" || n.Name == "" {
			t.Errorf("network with an empty ID or name: %+v", n)
		}
	}
}

func TestScanProducesAValidReport(t *testing.T) {
	client, ctx := connect(t)

	r, err := scanner.New(client, scanner.Config{
		IncludeResources: true,
		Tool:             model.ToolInfo{Name: "doctordock", Version: "integration"},
	}).Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if r.SchemaVersion != model.SchemaVersion {
		t.Errorf("schema version = %q", r.SchemaVersion)
	}
	if r.Score < 0 || r.Score > 100 {
		t.Errorf("score %d outside [0,100]", r.Score)
	}
	if r.Findings == nil {
		t.Error("Findings must never be nil")
	}

	for _, f := range r.Findings {
		if f.ID == "" || f.Rule == "" || f.Title == "" || f.Recommendation == "" {
			t.Errorf("incomplete finding: %+v", f)
		}
		if !f.Severity.Valid() {
			t.Errorf("finding %s has an invalid severity %q", f.ID, f.Severity)
		}
		if !f.Category.Valid() {
			t.Errorf("finding %s has an invalid category %q", f.ID, f.Category)
		}
	}

	// The summary must agree with the attached resource lists.
	if r.Summary.Containers.Total != len(r.Containers) {
		t.Errorf("summary says %d containers, %d attached",
			r.Summary.Containers.Total, len(r.Containers))
	}
	if r.Summary.Images.Total != len(r.Images) {
		t.Errorf("summary says %d images, %d attached",
			r.Summary.Images.Total, len(r.Images))
	}
}

// TestScanIsReadOnly verifies DoctorDock changes nothing. v0.1 must never
// mutate the environment it inspects.
func TestScanIsReadOnly(t *testing.T) {
	client, ctx := connect(t)
	s := scanner.New(client, scanner.Config{})

	before, err := s.Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := s.Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(before.Containers) != len(after.Containers) ||
		len(before.Images) != len(after.Images) ||
		len(before.Volumes) != len(after.Volumes) ||
		len(before.Networks) != len(after.Networks) {
		t.Fatal("a scan changed the number of Docker resources")
	}
}

func TestScanCompletesQuickly(t *testing.T) {
	client, ctx := connect(t)

	start := time.Now()
	if _, err := scanner.New(client, scanner.Config{}).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	// This is a tool people run dozens of times a day. Anything past a couple
	// of seconds on a normal machine means something regressed.
	if elapsed > 15*time.Second {
		t.Errorf("scan took %v", elapsed)
	}
	t.Logf("scan completed in %v", elapsed.Round(time.Millisecond))
}
