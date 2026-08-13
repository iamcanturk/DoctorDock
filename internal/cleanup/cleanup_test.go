package cleanup

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

var now = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) time.Time { return now.Add(-d) }

func env() *model.Environment {
	return &model.Environment{
		Containers: []model.Container{
			{ID: "c-run", Name: "running", State: model.StateRunning, Created: ago(30 * 24 * time.Hour)},
			{ID: "c-stop", Name: "stopped", State: model.StateExited, Status: "Exited (0) 3 days ago", Created: ago(30 * 24 * time.Hour)},
			{ID: "c-new", Name: "just-stopped", State: model.StateExited, Created: ago(time.Hour)},
			{ID: "c-paused", Name: "paused", State: model.StatePaused, Created: ago(30 * 24 * time.Hour)},
			{ID: "c-restart", Name: "restarting", State: model.StateRestarting, Created: ago(30 * 24 * time.Hour)},
		},
		Images: []model.Image{
			{ID: "i-live", RepoTags: []string{"live:1"}, Size: 100, InUse: true, UsedBy: []string{"running"}, Created: ago(30 * 24 * time.Hour)},
			// Referenced only by a container this cleanup would also remove.
			{ID: "i-freed", RepoTags: []string{"freed:1"}, Size: 200, InUse: true, UsedBy: []string{"stopped"}, Created: ago(30 * 24 * time.Hour)},
			{ID: "i-unused", RepoTags: []string{"unused:1"}, Size: 300, Created: ago(30 * 24 * time.Hour)},
			{ID: "i-dangling", Size: 50, Dangling: true, Created: ago(30 * 24 * time.Hour)},
			{ID: "i-new", RepoTags: []string{"fresh:1"}, Size: 400, Created: ago(time.Hour)},
		},
		Volumes: []model.Volume{
			{Name: "v-live", InUse: true, UsedBy: []string{"running"}, Created: ago(30 * 24 * time.Hour)},
			{Name: "v-orphan", Created: ago(30 * 24 * time.Hour)},
			{Name: "v-freed", InUse: true, UsedBy: []string{"stopped"}, Created: ago(30 * 24 * time.Hour)},
		},
		Networks: []model.Network{
			{ID: "n-bridge", Name: "bridge", Created: ago(30 * 24 * time.Hour)},
			{ID: "n-host", Name: "host", Created: ago(30 * 24 * time.Hour)},
			{ID: "n-live", Name: "app_net", Containers: []string{"running"}, Created: ago(30 * 24 * time.Hour)},
			{ID: "n-orphan", Name: "orphan_net", Created: ago(30 * 24 * time.Hour)},
			{ID: "n-freed", Name: "freed_net", Containers: []string{"stopped"}, Created: ago(30 * 24 * time.Hour)},
		},
	}
}

func plan(t *testing.T, opts Options) []model.CleanupItem {
	t.Helper()
	opts.Now = now
	return Plan(env(), opts)
}

func names(items []model.CleanupItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = string(item.Resource) + ":" + item.Name
	}
	return out
}

func has(items []model.CleanupItem, kind model.ResourceKind, name string) bool {
	for _, item := range items {
		if item.Resource == kind && item.Name == name {
			return true
		}
	}
	return false
}

// TestDefaultTargetsAreConservative pins what `doctordock cleanup` with no
// flags considers: only what Docker itself calls safe to prune.
func TestDefaultTargetsAreConservative(t *testing.T) {
	items := plan(t, Options{Targets: DefaultTargets()})

	if !has(items, model.ResourceImage, "i-dangling") {
		t.Error("dangling images should be in the default target set")
	}
	if !has(items, model.ResourceNetwork, "orphan_net") {
		t.Error("unused networks should be in the default target set")
	}
	for _, item := range items {
		if item.Resource == model.ResourceContainer || item.Resource == model.ResourceVolume {
			t.Errorf("default cleanup must not touch %s: %v", item.Resource, names(items))
		}
		if item.Risk != model.RiskSafe {
			t.Errorf("default cleanup should only include safe items, got %s for %s", item.Risk, item.Name)
		}
	}
}

// TestAllNeverIncludesVolumes is the single most important test in this
// package. --all covering volumes is how people lose databases.
func TestAllNeverIncludesVolumes(t *testing.T) {
	if All().Volumes {
		t.Fatal("All() must never select volumes")
	}

	items := plan(t, Options{Targets: All()})
	for _, item := range items {
		if item.Resource == model.ResourceVolume {
			t.Fatalf("--all planned a volume removal: %s", item.Name)
		}
	}
}

func TestVolumesRequireExplicitOptIn(t *testing.T) {
	items := plan(t, Options{Targets: Targets{Volumes: true}})

	if !has(items, model.ResourceVolume, "v-orphan") {
		t.Fatalf("--volumes should plan the orphan volume, got %v", names(items))
	}
	for _, item := range items {
		if item.Resource != model.ResourceVolume {
			t.Errorf("--volumes alone planned a %s", item.Resource)
		}
		if item.Risk != model.RiskDataLoss {
			t.Errorf("volume %s carries risk %s, want data-loss", item.Name, item.Risk)
		}
	}
}

func TestRunningResourcesAreNeverTouched(t *testing.T) {
	items := plan(t, Options{Targets: Targets{
		Containers: true, Images: true, Networks: true, Volumes: true,
	}})

	for _, forbidden := range []struct {
		kind model.ResourceKind
		name string
	}{
		{model.ResourceContainer, "running"},
		{model.ResourceContainer, "paused"},
		{model.ResourceContainer, "restarting"},
		{model.ResourceImage, "live:1"},
		{model.ResourceVolume, "v-live"},
		{model.ResourceNetwork, "app_net"},
	} {
		if has(items, forbidden.kind, forbidden.name) {
			t.Errorf("planned removal of in-use %s %q", forbidden.kind, forbidden.name)
		}
	}
}

func TestBuiltinNetworksAreNeverPlanned(t *testing.T) {
	items := plan(t, Options{Targets: Targets{Networks: true}})
	for _, name := range []string{"bridge", "host", "none", "ingress"} {
		if has(items, model.ResourceNetwork, name) {
			t.Errorf("planned removal of Docker's predefined network %q", name)
		}
	}
}

// TestCascadeAccountsForDoomedContainers is what stops the user having to run
// cleanup twice for it to converge.
func TestCascadeAccountsForDoomedContainers(t *testing.T) {
	// Without --containers, resources held by the stopped container stay.
	without := plan(t, Options{Targets: Targets{Images: true, Networks: true, Volumes: true}})
	if has(without, model.ResourceImage, "freed:1") {
		t.Error("an image held by a container that is staying must not be planned")
	}
	if has(without, model.ResourceNetwork, "freed_net") {
		t.Error("a network held by a container that is staying must not be planned")
	}

	// With --containers, the same resources become removable.
	with := plan(t, Options{Targets: Targets{
		Containers: true, Images: true, Networks: true, Volumes: true,
	}})
	if !has(with, model.ResourceImage, "freed:1") {
		t.Errorf("an image held only by a doomed container should be planned, got %v", names(with))
	}
	if !has(with, model.ResourceNetwork, "freed_net") {
		t.Error("a network held only by a doomed container should be planned")
	}
	if !has(with, model.ResourceVolume, "v-freed") {
		t.Error("a volume held only by a doomed container should be planned")
	}

	// The reason explains why, rather than just asserting it is unused.
	for _, item := range with {
		if item.Name == "freed:1" && !strings.Contains(item.Reason, "also removes") {
			t.Errorf("reason should explain the cascade, got %q", item.Reason)
		}
	}
}

func TestKeepSinceProtectsRecentWork(t *testing.T) {
	targets := Targets{Containers: true, Images: true, Networks: true}

	unprotected := plan(t, Options{Targets: targets})
	if !has(unprotected, model.ResourceImage, "fresh:1") {
		t.Fatal("the recent image should be planned without --keep-since")
	}
	if !has(unprotected, model.ResourceContainer, "just-stopped") {
		t.Fatal("the recently stopped container should be planned without --keep-since")
	}

	protected := plan(t, Options{Targets: targets, KeepSince: 24 * time.Hour})
	if has(protected, model.ResourceImage, "fresh:1") {
		t.Error("--keep-since 24h must protect an image built an hour ago")
	}
	if has(protected, model.ResourceContainer, "just-stopped") {
		t.Error("--keep-since 24h must protect a container stopped an hour ago")
	}
	// Older resources are still planned.
	if !has(protected, model.ResourceImage, "unused:1") {
		t.Error("--keep-since must not protect resources outside the window")
	}
}

func TestRiskLevels(t *testing.T) {
	items := plan(t, Options{Targets: Targets{
		Containers: true, Images: true, Networks: true, Volumes: true,
	}})

	want := map[string]model.Risk{
		"i-dangling": model.RiskSafe,
		"orphan_net": model.RiskSafe,
		"unused:1":   model.RiskReview,
		"stopped":    model.RiskReview,
		"v-orphan":   model.RiskDataLoss,
	}

	for _, item := range items {
		if expected, ok := want[item.Name]; ok && item.Risk != expected {
			t.Errorf("%s has risk %s, want %s", item.Name, item.Risk, expected)
		}
	}
}

func TestEmptyEnvironmentPlansNothing(t *testing.T) {
	got := Plan(&model.Environment{}, Options{Targets: All(), Now: now})
	if len(got) != 0 {
		t.Errorf("planned %v against an empty environment", names(got))
	}
}

// --- apply -------------------------------------------------------------------

func TestApplyOrdersRemovals(t *testing.T) {
	items := []model.CleanupItem{
		{Resource: model.ResourceVolume, ID: "v1", Name: "v1"},
		{Resource: model.ResourceImage, ID: "i1", Name: "img:1"},
		{Resource: model.ResourceNetwork, ID: "n1", Name: "n1"},
		{Resource: model.ResourceContainer, ID: "c1", Name: "c1"},
	}

	p := &docker.FakePruner{}
	got := Apply(context.Background(), p, items)

	// Containers must go first — an image or volume they hold cannot be
	// removed until they are gone. Volumes go last, so a failure earlier stops
	// before the irreversible part.
	want := []string{"container:c1", "image:i1", "network:n1", "volume:v1"}
	if len(p.Removed) != len(want) {
		t.Fatalf("removed %v, want %v", p.Removed, want)
	}
	for i := range want {
		if p.Removed[i] != want[i] {
			t.Fatalf("removal order was %v, want %v", p.Removed, want)
		}
	}

	for _, item := range got {
		if !item.Removed || item.Error != "" {
			t.Errorf("%s should be marked removed, got removed=%v error=%q",
				item.Name, item.Removed, item.Error)
		}
	}
}

func TestApplyRecordsFailuresWithoutAborting(t *testing.T) {
	items := []model.CleanupItem{
		{Resource: model.ResourceImage, ID: "i-ok", Name: "ok:1", Size: 100},
		{Resource: model.ResourceImage, ID: "i-bad", Name: "bad:1", Size: 200},
		{Resource: model.ResourceNetwork, ID: "n-ok", Name: "n-ok"},
	}

	p := &docker.FakePruner{Errors: map[string]error{"i-bad": errors.New("image is in use")}}
	got := Apply(context.Background(), p, items)

	summary := model.SummarizeCleanup(got)
	if summary.Removed != 2 || summary.Failed != 1 {
		t.Errorf("summary = %d removed, %d failed; want 2 and 1", summary.Removed, summary.Failed)
	}
	// A locked image must not prevent the network from being reclaimed.
	if len(p.Removed) != 2 {
		t.Errorf("a failure aborted the rest: %v", p.Removed)
	}
	// Only successful removals count toward reclaimed space.
	if summary.ReclaimedBytes != 100 {
		t.Errorf("reclaimed %d bytes, want 100 — a failed removal reclaimed nothing", summary.ReclaimedBytes)
	}

	for _, item := range got {
		if item.ID == "i-bad" && (item.Removed || item.Error == "") {
			t.Errorf("the failed item should carry the error and not be marked removed: %+v", item)
		}
	}
}

func TestApplyStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := &docker.FakePruner{}
	got := Apply(ctx, p, []model.CleanupItem{{Resource: model.ResourceImage, ID: "i1", Name: "i1"}})

	if len(p.Removed) != 0 {
		t.Errorf("a cancelled context still removed %v", p.Removed)
	}
	if got[0].Removed || got[0].Error == "" {
		t.Error("a cancelled item should be recorded as not removed, with the reason")
	}
}

func TestNewPlanNeverProducesNullItems(t *testing.T) {
	p := NewPlan(model.ToolInfo{Name: "doctordock"}, nil, false, now)
	if p.Items == nil {
		t.Error("Items must be an empty slice, never nil — clients iterate it directly")
	}
	if p.Applied {
		t.Error("a plan built with applied=false must not claim to have been applied")
	}
	if p.SchemaVersion != model.SchemaVersion {
		t.Errorf("schema version = %q", p.SchemaVersion)
	}
}
