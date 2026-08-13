package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		in      string
		want    model.Severity
		wantErr bool
	}{
		{"high", model.SeverityHigh, false},
		{"HIGH", model.SeverityHigh, false},
		{"  Critical  ", model.SeverityCritical, false},
		{"info", model.SeverityInfo, false},
		{"", "", true},
		{"catastrophic", "", true},
		{"warn", "", true},
	}

	for _, tt := range tests {
		got, err := model.ParseSeverity(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseSeverity(%q) = %q, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSeverity(%q) returned %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseSeverity(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSeverityRankOrdering(t *testing.T) {
	// --fail-on comparisons depend on this order being strictly increasing.
	for i := 1; i < len(model.AllSeverities); i++ {
		lower, higher := model.AllSeverities[i-1], model.AllSeverities[i]
		if lower.Rank() >= higher.Rank() {
			t.Errorf("%s rank %d is not below %s rank %d",
				lower, lower.Rank(), higher, higher.Rank())
		}
	}
}

func TestTagOf(t *testing.T) {
	tests := []struct{ ref, want string }{
		{"nginx:1.25", "1.25"},
		{"nginx", ""},
		{"library/nginx:alpine", "alpine"},
		// A colon in the registry host is a port, not a tag.
		{"registry.local:5000/app", ""},
		{"registry.local:5000/app:v2", "v2"},
		{"nginx@sha256:abc123", ""},
		{"registry.local:5000/app@sha256:abc", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := model.TagOf(tt.ref); got != tt.want {
			t.Errorf("TagOf(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestRepositoryOf(t *testing.T) {
	tests := []struct{ ref, want string }{
		{"nginx:1.25", "nginx"},
		{"nginx", "nginx"},
		{"registry.local:5000/app", "registry.local:5000/app"},
		{"registry.local:5000/app:v2", "registry.local:5000/app"},
		{"nginx@sha256:abc", "nginx"},
	}

	for _, tt := range tests {
		if got := model.RepositoryOf(tt.ref); got != tt.want {
			t.Errorf("RepositoryOf(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestIsMutableTag(t *testing.T) {
	mutable := []string{"nginx", "nginx:latest", "app:main", "app:dev", "svc:nightly"}
	for _, ref := range mutable {
		if !model.IsMutableTag(ref) {
			t.Errorf("IsMutableTag(%q) = false, want true", ref)
		}
	}

	immutable := []string{
		"nginx:1.25.3",
		"nginx@sha256:abc123",
		// A digest pin is immutable even when a mutable-looking tag is present.
		"app:latest@sha256:abc123",
		"registry.local:5000/app:v2",
		"",
	}
	for _, ref := range immutable {
		if model.IsMutableTag(ref) {
			t.Errorf("IsMutableTag(%q) = true, want false", ref)
		}
	}
}

func TestContainerRunsAsRoot(t *testing.T) {
	tests := []struct {
		user string
		want bool
	}{
		{"", true},
		{"root", true},
		{"0", true},
		{"0:0", true},
		{"1000", false},
		{"1000:1000", false},
		{"appuser", false},
		{"  ", true},
	}

	for _, tt := range tests {
		c := model.Container{EffectiveUser: tt.user}
		if got := c.RunsAsRoot(); got != tt.want {
			t.Errorf("EffectiveUser=%q RunsAsRoot() = %v, want %v", tt.user, got, tt.want)
		}
	}
}

func TestPortBinding(t *testing.T) {
	tests := []struct {
		name              string
		port              model.Port
		published, public bool
	}{
		{"exposed only", model.Port{PrivatePort: 80}, false, false},
		{"wildcard v4", model.Port{PrivatePort: 80, PublicPort: 8080, HostIP: "0.0.0.0"}, true, true},
		{"wildcard v6", model.Port{PrivatePort: 80, PublicPort: 8080, HostIP: "::"}, true, true},
		{"empty host ip", model.Port{PrivatePort: 80, PublicPort: 8080}, true, true},
		{"loopback", model.Port{PrivatePort: 80, PublicPort: 8080, HostIP: "127.0.0.1"}, true, false},
		{"specific ip", model.Port{PrivatePort: 80, PublicPort: 8080, HostIP: "192.168.1.5"}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.port.IsPublished(); got != tt.published {
				t.Errorf("IsPublished() = %v, want %v", got, tt.published)
			}
			if got := tt.port.IsPublicallyBound(); got != tt.public {
				t.Errorf("IsPublicallyBound() = %v, want %v", got, tt.public)
			}
		})
	}
}

func TestVolumeIsAnonymous(t *testing.T) {
	const hex64 = "156aa8105ebc31baa8874d56fd5eb1fc70999e8c7114d352eb129d0b91782569"

	labelled := model.Volume{
		Name:   "my-data",
		Labels: map[string]string{model.AnonymousLabel: ""},
	}
	if !labelled.IsAnonymous() {
		t.Error("volume with the anonymous label should be anonymous")
	}

	if !(model.Volume{Name: hex64}).IsAnonymous() {
		t.Error("64-hex volume name should be detected as anonymous")
	}

	named := []model.Volume{
		{Name: "postgres-data"},
		{Name: "short"},
		// 64 characters, but not hex.
		{Name: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
	}
	for _, v := range named {
		if v.IsAnonymous() {
			t.Errorf("volume %q should not be anonymous", v.Name)
		}
	}
}

func TestNetworkIsBuiltin(t *testing.T) {
	for _, name := range []string{"bridge", "host", "none", "ingress"} {
		if !(model.Network{Name: name}).IsBuiltin() {
			t.Errorf("%q should be builtin", name)
		}
	}
	if (model.Network{Name: "myapp_default"}).IsBuiltin() {
		t.Error("user-defined network should not be builtin")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1.0 kB"},
		{1_500_000_000, "1.5 GB"},
		{2_800_000_000, "2.8 GB"},
		{-1, "unknown"},
	}
	for _, tt := range tests {
		if got := model.FormatBytes(tt.in); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	env := &model.Environment{
		Containers: []model.Container{
			{Name: "a", State: model.StateRunning},
			{Name: "b", State: model.StateRunning, Health: model.HealthUnhealthy},
			{Name: "c", State: model.StateExited},
			{Name: "d", State: model.StateRestarting},
			{Name: "e", State: model.StatePaused},
			// Unhealthy but stopped: the status is stale, not a live problem.
			{Name: "f", State: model.StateExited, Health: model.HealthUnhealthy},
		},
		Images: []model.Image{
			{ID: "1", Size: 100, InUse: true},
			{ID: "2", Size: 200},
			{ID: "3", Size: 50, Dangling: true},
		},
		Volumes: []model.Volume{
			{Name: "used", InUse: true},
			{Name: "unused"},
		},
		Networks: []model.Network{
			{Name: "bridge"},
			{Name: "app_net", Containers: []string{"a"}},
			{Name: "orphan_net"},
		},
	}

	findings := []model.Finding{
		{ID: "DD001", Severity: model.SeverityHigh, Category: model.CategorySecurity},
		{ID: "DD002", Severity: model.SeverityCritical, Category: model.CategorySecurity},
		{ID: "DD015", Severity: model.SeverityInfo, Category: model.CategoryCleanup},
	}

	s := model.Summarize(env, findings)

	if s.Containers.Running != 2 || s.Containers.Stopped != 2 ||
		s.Containers.Restarting != 1 || s.Containers.Paused != 1 {
		t.Errorf("container states = %+v", s.Containers)
	}
	if s.Containers.Unhealthy != 1 {
		t.Errorf("Unhealthy = %d, want 1 (stopped containers keep a stale status)", s.Containers.Unhealthy)
	}

	if s.Images.Total != 3 || s.Images.Unused != 2 || s.Images.Dangling != 1 {
		t.Errorf("image counts = %+v", s.Images)
	}
	if s.Images.TotalSize != 350 {
		t.Errorf("TotalSize = %d, want 350", s.Images.TotalSize)
	}
	if s.Images.ReclaimableSize != 250 {
		t.Errorf("ReclaimableSize = %d, want 250", s.Images.ReclaimableSize)
	}

	if s.Volumes.Unused != 1 {
		t.Errorf("unused volumes = %d, want 1", s.Volumes.Unused)
	}

	if s.Networks.Custom != 2 || s.Networks.Unused != 1 {
		t.Errorf("network counts = %+v (builtin networks must never count as unused)", s.Networks)
	}

	if s.Findings.BySeverity.High != 1 || s.Findings.BySeverity.Critical != 1 {
		t.Errorf("severity counts = %+v", s.Findings.BySeverity)
	}
	if s.Findings.ByCategory[model.CategorySecurity] != 2 {
		t.Errorf("security findings = %d, want 2", s.Findings.ByCategory[model.CategorySecurity])
	}
	// Every category is present even at zero, so clients can index safely.
	if _, ok := s.Findings.ByCategory[model.CategoryPerformance]; !ok {
		t.Error("ByCategory should contain every category, including empty ones")
	}
}

func TestHighestSeverity(t *testing.T) {
	if _, ok := model.HighestSeverity(nil); ok {
		t.Error("empty findings should report no highest severity")
	}

	got, ok := model.HighestSeverity([]model.Finding{
		{Severity: model.SeverityLow},
		{Severity: model.SeverityCritical},
		{Severity: model.SeverityMedium},
	})
	if !ok || got != model.SeverityCritical {
		t.Errorf("HighestSeverity = %q (%v), want CRITICAL", got, ok)
	}
}

func TestReportFindingsBySeverity(t *testing.T) {
	r := &model.Report{Findings: []model.Finding{
		{ID: "a", Severity: model.SeverityLow},
		{ID: "b", Severity: model.SeverityCritical},
		{ID: "c", Severity: model.SeverityLow},
		{ID: "d", Severity: model.SeverityHigh},
	}}

	got := r.FindingsBySeverity()
	want := []string{"b", "d", "a", "c"}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("order = %v, want %v", ids(got), want)
		}
	}
}

func ids(findings []model.Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = f.ID
	}
	return out
}

// TestReportJSONContract guards the field names non-Go clients decode. A
// rename here is a breaking change requiring a schema_version bump, so it
// should fail loudly rather than silently break the macOS app.
func TestReportJSONContract(t *testing.T) {
	r := model.Report{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		Tool:          model.ToolInfo{Name: "doctordock", Version: "0.1.0"},
		Score:         78,
		Findings: []model.Finding{{
			ID:           "DD005",
			Rule:         "Docker socket exposed",
			Severity:     model.SeverityCritical,
			Category:     model.CategorySecurity,
			Resource:     model.ResourceContainer,
			ResourceID:   "abc123",
			ResourceName: "api",
			Title:        "Docker socket is mounted into the container",
		}},
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{
		"schema_version", "generated_at", "tool", "docker", "score", "summary", "findings",
	} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("report JSON is missing required key %q", key)
		}
	}

	finding := decoded["findings"].([]any)[0].(map[string]any)
	for _, key := range []string{
		"id", "rule", "severity", "category", "resource", "resource_id", "resource_name",
		"title", "description", "recommendation",
	} {
		if _, ok := finding[key]; !ok {
			t.Errorf("finding JSON is missing required key %q", key)
		}
	}

	if finding["severity"] != "CRITICAL" {
		t.Errorf("severity encoded as %v, want the string CRITICAL", finding["severity"])
	}
}

// TestFindingsNeverNull matters because clients iterate the array directly.
func TestFindingsNeverNull(t *testing.T) {
	r := model.Report{Findings: []model.Finding{}}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["findings"] == nil {
		t.Error("findings encoded as null; clients expect an array")
	}
}
