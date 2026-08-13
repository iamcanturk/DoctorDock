package scanner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func fakeClient() *docker.Fake {
	return &docker.Fake{
		DockerInfo: model.DockerInfo{ServerVersion: "28.0.0", OSType: "linux", Architecture: "arm64"},
		Containers: []model.Container{
			{
				ID:             "c1",
				Name:           "api",
				Image:          "api:latest",
				ImageID:        "sha256:img1",
				State:          model.StateRunning,
				EffectiveUser:  "root",
				HasHealthcheck: true,
				RestartPolicy:  "always",
				MemoryLimit:    512 << 20,
				Networks:       []string{"app_net"},
				Mounts: []model.Mount{
					{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"},
					{Type: "volume", Name: "app-data", Destination: "/data"},
				},
			},
			{
				ID:             "c2",
				Name:           "db",
				Image:          "postgres:16",
				ImageID:        "sha256:img2",
				State:          model.StateRunning,
				EffectiveUser:  "postgres",
				HasHealthcheck: true,
				RestartPolicy:  "always",
				MemoryLimit:    1 << 30,
				Networks:       []string{"app_net"},
			},
		},
		Images: []model.Image{
			{ID: "sha256:img1", RepoTags: []string{"api:latest"}, Size: 100 << 20},
			{ID: "sha256:img2", RepoTags: []string{"postgres:16"}, Size: 300 << 20},
			{ID: "sha256:img3", RepoTags: []string{"old:1.0"}, Size: 50 << 20},
			{ID: "sha256:img4", Size: 10 << 20},
		},
		Volumes: []model.Volume{
			{Name: "app-data", Driver: "local"},
			{Name: "orphan", Driver: "local"},
		},
		Networks: []model.Network{
			{ID: "n0", Name: "bridge", Driver: "bridge"},
			{ID: "n1", Name: "app_net", Driver: "bridge"},
			{ID: "n2", Name: "stale_net", Driver: "bridge"},
		},
	}
}

func newScanner(t *testing.T, client docker.Client, cfg Config) *Scanner {
	t.Helper()
	if cfg.Now.IsZero() {
		cfg.Now = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	}
	return New(client, cfg)
}

func TestCollectResolvesRelationships(t *testing.T) {
	s := newScanner(t, fakeClient(), Config{})

	env, err := s.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]model.Image{}
	for _, img := range env.Images {
		byID[img.ID] = img
	}

	if !byID["sha256:img1"].InUse || !byID["sha256:img2"].InUse {
		t.Error("images referenced by containers should be marked in use")
	}
	if byID["sha256:img3"].InUse || byID["sha256:img4"].InUse {
		t.Error("unreferenced images should not be marked in use")
	}
	if len(byID["sha256:img1"].UsedBy) != 1 || byID["sha256:img1"].UsedBy[0] != "api" {
		t.Errorf("UsedBy = %v, want [api]", byID["sha256:img1"].UsedBy)
	}

	for _, v := range env.Volumes {
		want := v.Name == "app-data"
		if v.InUse != want {
			t.Errorf("volume %s InUse = %v, want %v", v.Name, v.InUse, want)
		}
	}

	for _, n := range env.Networks {
		want := 0
		if n.Name == "app_net" {
			want = 2
		}
		if len(n.Containers) != want {
			t.Errorf("network %s has %d containers, want %d", n.Name, len(n.Containers), want)
		}
	}
}

// TestLinkMatchesImagesByReference covers the case where a tag was rebuilt:
// the container still points at the old image ID while the tag names a new
// one. Both must be considered in use.
func TestLinkMatchesImagesByReference(t *testing.T) {
	env := &model.Environment{
		Containers: []model.Container{
			{Name: "app", Image: "app:latest", ImageID: "sha256:old"},
			// `docker run nginx` records the reference without a tag.
			{Name: "web", Image: "nginx", ImageID: "sha256:nginx"},
		},
		Images: []model.Image{
			{ID: "sha256:old", RepoTags: nil, Dangling: true},
			{ID: "sha256:new", RepoTags: []string{"app:latest"}},
			{ID: "sha256:nginx", RepoTags: []string{"nginx:latest"}},
			{ID: "sha256:other", RepoTags: []string{"other:1"}},
		},
	}

	link(env)

	want := map[string]bool{
		"sha256:old":   true,
		"sha256:new":   true,
		"sha256:nginx": true,
		"sha256:other": false,
	}
	for _, img := range env.Images {
		if img.InUse != want[img.ID] {
			t.Errorf("image %s InUse = %v, want %v", img.ID, img.InUse, want[img.ID])
		}
	}
}

func TestScanProducesCompleteReport(t *testing.T) {
	s := newScanner(t, fakeClient(), Config{
		IncludeResources: true,
		Tool:             model.ToolInfo{Name: "doctordock", Version: "0.1.0"},
	})

	r, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if r.SchemaVersion != model.SchemaVersion {
		t.Errorf("schema version = %q", r.SchemaVersion)
	}
	if r.Findings == nil {
		t.Error("Findings must be an empty slice, never nil")
	}
	if r.Score < 0 || r.Score > 100 {
		t.Errorf("score %d outside [0,100]", r.Score)
	}
	if len(r.Containers) != 2 || len(r.Images) != 4 {
		t.Error("IncludeResources should attach the resource lists")
	}

	// The Docker socket mount is the worst thing in the fixture.
	highest, ok := r.HighestSeverity()
	if !ok || highest != model.SeverityCritical {
		t.Errorf("highest severity = %q, want CRITICAL", highest)
	}

	var sawSocket bool
	for _, f := range r.Findings {
		if f.ID == "DD005" && f.ResourceName == "api" {
			sawSocket = true
		}
	}
	if !sawSocket {
		t.Error("DD005 should fire for the container mounting the Docker socket")
	}
}

func TestScanOmitsResourcesByDefault(t *testing.T) {
	s := newScanner(t, fakeClient(), Config{})
	r, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Containers != nil || r.Images != nil || r.Volumes != nil || r.Networks != nil {
		t.Error("resource lists should be omitted unless IncludeResources is set")
	}
	if r.Summary.Containers.Total != 2 {
		t.Error("the summary must still be populated")
	}
}

func TestFindingsAreSortedMostSevereFirst(t *testing.T) {
	s := newScanner(t, fakeClient(), Config{})
	r, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i < len(r.Findings); i++ {
		prev, cur := r.Findings[i-1], r.Findings[i]
		if prev.Severity.Rank() < cur.Severity.Rank() {
			t.Fatalf("finding %d (%s) is less severe than %d (%s)",
				i-1, prev.Severity, i, cur.Severity)
		}
		if prev.Severity == cur.Severity && prev.ID > cur.ID {
			t.Fatalf("within a severity, rule IDs should ascend: %s before %s", prev.ID, cur.ID)
		}
	}
}

func TestScanIsDeterministic(t *testing.T) {
	first, err := newScanner(t, fakeClient(), Config{}).Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := newScanner(t, fakeClient(), Config{}).Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if first.Score != second.Score || len(first.Findings) != len(second.Findings) {
		t.Fatal("two scans of the same environment disagree")
	}
	for i := range first.Findings {
		a, b := first.Findings[i], second.Findings[i]
		if a.ID != b.ID || a.ResourceID != b.ResourceID {
			t.Fatalf("finding %d differs between runs: %s/%s vs %s/%s",
				i, a.ID, a.ResourceID, b.ID, b.ResourceID)
		}
	}
}

func TestIgnoredRulesAreSkippedAndRecorded(t *testing.T) {
	s := newScanner(t, fakeClient(), Config{IgnoreRules: []string{"DD005", "DD001"}})
	r, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range r.Findings {
		if f.ID == "DD005" || f.ID == "DD001" {
			t.Fatalf("ignored rule %s still produced a finding", f.ID)
		}
	}

	// A reader must be able to tell "clean" apart from "not checked".
	if len(r.SkippedRules) != 2 {
		t.Errorf("SkippedRules = %v, want both ignored rules recorded", r.SkippedRules)
	}
}

func TestRuleSubsetIsHonoured(t *testing.T) {
	only, _ := rules.ByID("DD005")
	s := newScanner(t, fakeClient(), Config{Rules: []rules.Rule{only}})

	r, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Findings {
		if f.ID != "DD005" {
			t.Fatalf("unexpected finding %s when only DD005 was enabled", f.ID)
		}
	}
	if len(r.Findings) == 0 {
		t.Fatal("DD005 should have fired")
	}
}

func TestCollectSurfacesClientErrors(t *testing.T) {
	sentinel := errors.New("daemon exploded")
	client := fakeClient()
	client.ContainersErr = sentinel

	_, err := newScanner(t, client, Config{}).Scan(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
}

func TestCollectCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := newScanner(t, fakeClient(), Config{})
	env, err := s.Collect(ctx)
	if err != nil {
		// A cancelled context may fail during collection, which is fine.
		return
	}
	if _, err := s.Evaluate(ctx, env); !errors.Is(err, context.Canceled) {
		t.Fatalf("Evaluate on a cancelled context returned %v, want context.Canceled", err)
	}
}
