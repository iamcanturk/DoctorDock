package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// run evaluates one rule against an environment and returns its findings.
func run(t *testing.T, r Rule, env *model.Environment) []model.Finding {
	t.Helper()
	return r.Check(context.Background(), Target{
		Environment: env,
		Options:     DefaultOptions(),
	})
}

func envWith(containers ...model.Container) *model.Environment {
	return &model.Environment{Containers: containers}
}

// assertFindings checks the count and, when findings are expected, that each
// carries the fields every renderer relies on.
func assertFindings(t *testing.T, got []model.Finding, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("got %d findings, want %d: %v", len(got), want, titles(got))
	}
	for _, f := range got {
		if f.ID == "" || f.Rule == "" || f.Title == "" || f.Recommendation == "" {
			t.Errorf("finding %+v is missing a required field", f)
		}
		if f.ResourceName == "" {
			t.Errorf("finding %s has no resource name", f.ID)
		}
	}
}

func titles(findings []model.Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = f.ID + ":" + f.ResourceName
	}
	return out
}

// --- registry ---------------------------------------------------------------

func TestRegistryIsValid(t *testing.T) {
	if err := Validate(All()); err != nil {
		t.Fatal(err)
	}
	if len(All()) == 0 {
		t.Fatal("registry is empty")
	}
}

func TestAllIsSortedAndStable(t *testing.T) {
	all := All()
	for i := 1; i < len(all); i++ {
		if all[i-1].ID() >= all[i].ID() {
			t.Fatalf("rules are not sorted by ID: %s before %s", all[i-1].ID(), all[i].ID())
		}
	}

	// All must return a copy; mutating it must not corrupt the registry.
	all[0] = nil
	if All()[0] == nil {
		t.Fatal("All() exposes the registry slice; callers can corrupt it")
	}
}

func TestByID(t *testing.T) {
	if r, ok := ByID("DD005"); !ok || r.Name() == "" {
		t.Fatal("DD005 should resolve")
	}
	if _, ok := ByID("DD999"); ok {
		t.Fatal("unknown rule should not resolve")
	}
}

// TestEveryRuleHandlesEmptyEnvironment catches nil-map and nil-slice panics
// that only show up on a machine with nothing running.
func TestEveryRuleHandlesEmptyEnvironment(t *testing.T) {
	empty := &model.Environment{}
	for _, r := range All() {
		findings := run(t, r, empty)
		if len(findings) != 0 {
			t.Errorf("%s produced %d findings on an empty environment", r.ID(), len(findings))
		}
	}
}

// --- DD001 ------------------------------------------------------------------

func TestRootUser(t *testing.T) {
	env := envWith(
		model.Container{Name: "root-explicit", EffectiveUser: "root"},
		model.Container{Name: "root-implicit", EffectiveUser: ""},
		model.Container{Name: "uid-zero", EffectiveUser: "0:0"},
		model.Container{Name: "non-root", EffectiveUser: "1000:1000"},
		model.Container{Name: "named-user", EffectiveUser: "appuser"},
	)

	got := run(t, RootUser{}, env)
	assertFindings(t, got, 3)

	for _, f := range got {
		if f.Details["effective_user"] == "" {
			t.Errorf("%s should record the effective user it resolved", f.ResourceName)
		}
	}
}

// --- DD002 ------------------------------------------------------------------

func TestPrivilegedContainer(t *testing.T) {
	env := envWith(
		model.Container{Name: "priv", Privileged: true},
		model.Container{Name: "normal"},
	)

	got := run(t, PrivilegedContainer{}, env)
	assertFindings(t, got, 1)
	if got[0].Severity != model.SeverityCritical {
		t.Errorf("severity = %s, want CRITICAL", got[0].Severity)
	}
}

// --- DD003 ------------------------------------------------------------------

func TestHostNetwork(t *testing.T) {
	env := envWith(
		model.Container{Name: "host-net", NetworkMode: "host"},
		model.Container{Name: "bridge-net", NetworkMode: "bridge"},
		model.Container{Name: "custom", NetworkMode: "app_default"},
	)
	assertFindings(t, run(t, HostNetwork{}, env), 1)
}

// --- DD004 ------------------------------------------------------------------

func TestSensitiveHostMount(t *testing.T) {
	tests := []struct {
		name     string
		mount    model.Mount
		want     bool
		severity model.Severity
	}{
		{"host root writable", model.Mount{Type: "bind", Source: "/", Destination: "/host"}, true, model.SeverityCritical},
		{"host root read-only", model.Mount{Type: "bind", Source: "/", Destination: "/host", ReadOnly: true}, true, model.SeverityHigh},
		{"etc", model.Mount{Type: "bind", Source: "/etc/passwd", Destination: "/etc/passwd"}, true, model.SeverityHigh},
		{"docker state", model.Mount{Type: "bind", Source: "/var/lib/docker", Destination: "/d"}, true, model.SeverityHigh},
		{"proc", model.Mount{Type: "bind", Source: "/proc", Destination: "/host/proc"}, true, model.SeverityHigh},
		// Docker Desktop rewrites macOS bind sources; without normalization
		// no sensitive path would ever match on a Mac.
		{"docker desktop ssh", model.Mount{Type: "bind", Source: "/host_mnt/Users/me/.ssh", Destination: "/root/.ssh"}, true, model.SeverityHigh},
		{"aws credentials", model.Mount{Type: "bind", Source: "/Users/me/.aws", Destination: "/root/.aws"}, true, model.SeverityHigh},
		{"windows desktop", model.Mount{Type: "bind", Source: "/run/desktop/mnt/host/c/Users/me/.kube", Destination: "/kube"}, true, model.SeverityHigh},

		{"ordinary project dir", model.Mount{Type: "bind", Source: "/Users/me/code/app", Destination: "/app"}, false, ""},
		{"named volume", model.Mount{Type: "volume", Name: "data", Source: "/var/lib/docker/volumes/data/_data", Destination: "/data"}, false, ""},
		// A path that merely starts with the same letters is not a match.
		{"etcd is not etc", model.Mount{Type: "bind", Source: "/etcd-data", Destination: "/data"}, false, ""},
		// The socket has its own rule; DD004 must not double-report it.
		{"docker socket", model.Mount{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := envWith(model.Container{Name: "c", Mounts: []model.Mount{tt.mount}})
			got := run(t, SensitiveHostMount{}, env)

			if !tt.want {
				assertFindings(t, got, 0)
				return
			}
			assertFindings(t, got, 1)
			if got[0].Severity != tt.severity {
				t.Errorf("severity = %s, want %s", got[0].Severity, tt.severity)
			}
		})
	}
}

// --- DD005 ------------------------------------------------------------------

func TestDockerSocketMount(t *testing.T) {
	tests := []struct {
		name  string
		mount model.Mount
		want  int
	}{
		{"unix socket", model.Mount{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"}, 1},
		// Read-only does not help: the socket is an API endpoint, not a file.
		{"read-only socket", model.Mount{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock", ReadOnly: true}, 1},
		{"remapped destination", model.Mount{Type: "bind", Source: "/var/run/docker.sock", Destination: "/tmp/d.sock"}, 1},
		{"windows named pipe", model.Mount{Type: "bind", Source: `\\.\pipe\docker_engine`, Destination: `\\.\pipe\docker_engine`}, 1},
		{"unrelated socket", model.Mount{Type: "bind", Source: "/tmp/app.sock", Destination: "/tmp/app.sock"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := envWith(model.Container{Name: "c", Mounts: []model.Mount{tt.mount}})
			got := run(t, DockerSocketMount{}, env)
			assertFindings(t, got, tt.want)
			if tt.want > 0 && got[0].Severity != model.SeverityCritical {
				t.Errorf("severity = %s, want CRITICAL", got[0].Severity)
			}
		})
	}
}

// --- DD006 ------------------------------------------------------------------

func TestExposedSensitivePort(t *testing.T) {
	tests := []struct {
		name     string
		port     model.Port
		want     int
		severity model.Severity
	}{
		{"mysql wildcard", model.Port{PrivatePort: 3306, PublicPort: 3307, Type: "tcp", HostIP: "0.0.0.0"}, 1, model.SeverityMedium},
		{"postgres wildcard", model.Port{PrivatePort: 5432, PublicPort: 5432, Type: "tcp", HostIP: "0.0.0.0"}, 1, model.SeverityMedium},
		{"docker api wildcard", model.Port{PrivatePort: 2375, PublicPort: 2375, Type: "tcp", HostIP: "0.0.0.0"}, 1, model.SeverityCritical},
		// Loopback is the recommended way to expose a database locally.
		{"mysql on loopback", model.Port{PrivatePort: 3306, PublicPort: 3306, Type: "tcp", HostIP: "127.0.0.1"}, 0, ""},
		// Exposed but not published is not reachable from anywhere.
		{"mysql not published", model.Port{PrivatePort: 3306, Type: "tcp"}, 0, ""},
		{"http wildcard", model.Port{PrivatePort: 80, PublicPort: 8080, Type: "tcp", HostIP: "0.0.0.0"}, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := envWith(model.Container{Name: "db", Ports: []model.Port{tt.port}})
			got := run(t, ExposedSensitivePort{}, env)
			assertFindings(t, got, tt.want)
			if tt.want > 0 && got[0].Severity != tt.severity {
				t.Errorf("severity = %s, want %s", got[0].Severity, tt.severity)
			}
		})
	}
}

// --- DD007 / DD008 ----------------------------------------------------------

func TestMissingHealthcheck(t *testing.T) {
	env := envWith(
		model.Container{Name: "with-hc", HasHealthcheck: true},
		model.Container{Name: "without-hc"},
	)
	got := run(t, MissingHealthcheck{}, env)
	assertFindings(t, got, 1)
	if got[0].ResourceName != "without-hc" {
		t.Errorf("flagged %q", got[0].ResourceName)
	}
}

func TestMissingRestartPolicy(t *testing.T) {
	env := envWith(
		model.Container{Name: "none"},
		model.Container{Name: "explicit-no", RestartPolicy: "no"},
		model.Container{Name: "always", RestartPolicy: "always"},
		model.Container{Name: "unless-stopped", RestartPolicy: "unless-stopped"},
	)
	assertFindings(t, run(t, MissingRestartPolicy{}, env), 2)
}

// --- DD009 ------------------------------------------------------------------

func TestDangerousCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		c        model.Container
		want     int
		severity model.Severity
	}{
		{"sys_admin", model.Container{Name: "a", CapAdd: []string{"SYS_ADMIN"}}, 1, model.SeverityCritical},
		{"net_admin", model.Container{Name: "b", CapAdd: []string{"NET_ADMIN"}}, 1, model.SeverityHigh},
		{"all", model.Container{Name: "c", CapAdd: []string{"ALL"}}, 1, model.SeverityCritical},
		{"harmless", model.Container{Name: "d", CapAdd: []string{"NET_BIND_SERVICE"}}, 0, ""},
		{"none", model.Container{Name: "e"}, 0, ""},
		// A privileged container already holds every capability and is
		// reported by DD002; repeating it here would double-count.
		{"privileged", model.Container{Name: "f", Privileged: true, CapAdd: []string{"SYS_ADMIN"}}, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, DangerousCapabilities{}, envWith(tt.c))
			assertFindings(t, got, tt.want)
			if tt.want > 0 && got[0].Severity != tt.severity {
				t.Errorf("severity = %s, want %s", got[0].Severity, tt.severity)
			}
		})
	}
}

// --- DD010 ------------------------------------------------------------------

func TestNoMemoryLimit(t *testing.T) {
	env := envWith(
		model.Container{Name: "running-unlimited", State: model.StateRunning},
		model.Container{Name: "running-limited", State: model.StateRunning, MemoryLimit: 512 << 20},
		model.Container{Name: "restarting", State: model.StateRestarting},
		// A stopped container consumes no memory.
		model.Container{Name: "stopped", State: model.StateExited},
	)
	got := run(t, NoMemoryLimit{}, env)
	assertFindings(t, got, 2)
}

// --- DD011 ------------------------------------------------------------------

func TestMutableImageTag(t *testing.T) {
	env := envWith(
		model.Container{Name: "latest", Image: "app:latest"},
		model.Container{Name: "implicit", Image: "nginx"},
		model.Container{Name: "pinned", Image: "nginx:1.25.3"},
		model.Container{Name: "digest", Image: "nginx@sha256:abc"},
	)
	assertFindings(t, run(t, MutableImageTag{}, env), 2)
}

// --- DD012 / DD013 ----------------------------------------------------------

func TestUnhealthyContainer(t *testing.T) {
	env := envWith(
		model.Container{Name: "sick", State: model.StateRunning, Health: model.HealthUnhealthy},
		model.Container{Name: "well", State: model.StateRunning, Health: model.HealthHealthy},
		// A stopped container keeps its last health status; that is history,
		// not a current failure.
		model.Container{Name: "stopped-sick", State: model.StateExited, Health: model.HealthUnhealthy},
	)
	got := run(t, UnhealthyContainer{}, env)
	assertFindings(t, got, 1)
	if got[0].ResourceName != "sick" {
		t.Errorf("flagged %q, want sick", got[0].ResourceName)
	}
}

func TestRestartLoop(t *testing.T) {
	env := envWith(
		model.Container{Name: "looping", State: model.StateRunning, RestartCount: 12},
		model.Container{Name: "stuck", State: model.StateRestarting, RestartCount: 1},
		model.Container{Name: "occasional", State: model.StateRunning, RestartCount: 2},
		model.Container{Name: "stable", State: model.StateRunning},
	)
	got := run(t, RestartLoop{}, env)
	assertFindings(t, got, 2)
}

func TestRestartLoopThresholdIsConfigurable(t *testing.T) {
	env := envWith(model.Container{Name: "c", State: model.StateRunning, RestartCount: 3})

	target := Target{Environment: env, Options: Options{RestartLoopThreshold: 3}.Normalize()}
	if got := (RestartLoop{}).Check(context.Background(), target); len(got) != 1 {
		t.Fatalf("threshold 3 with 3 restarts: got %d findings, want 1", len(got))
	}

	assertFindings(t, run(t, RestartLoop{}, env), 0)
}

// --- DD014 / DD015 / DD016 --------------------------------------------------

func TestImageCleanupRules(t *testing.T) {
	env := &model.Environment{Images: []model.Image{
		{ID: "sha256:a", RepoTags: []string{"app:1.0"}, InUse: true, Size: 100},
		{ID: "sha256:b", RepoTags: []string{"old:1.0"}, Size: 200},
		{ID: "sha256:c", Dangling: true, Size: 50},
		// A dangling image that is somehow still referenced is not garbage.
		{ID: "sha256:d", Dangling: true, InUse: true, Size: 10},
	}}

	dangling := run(t, DanglingImage{}, env)
	assertFindings(t, dangling, 1)
	if dangling[0].ResourceID != "sha256:c" {
		t.Errorf("dangling flagged %s", dangling[0].ResourceID)
	}

	// Dangling images are also unused; DD014 owns them so a single leftover
	// is not reported twice.
	unused := run(t, UnusedImage{}, env)
	assertFindings(t, unused, 1)
	if unused[0].ResourceID != "sha256:b" {
		t.Errorf("unused flagged %s", unused[0].ResourceID)
	}
}

func TestOversizedImage(t *testing.T) {
	env := &model.Environment{Images: []model.Image{
		{ID: "sha256:big", RepoTags: []string{"monolith:1"}, Size: 3_000_000_000},
		{ID: "sha256:small", RepoTags: []string{"svc:1"}, Size: 20_000_000},
	}}

	got := run(t, OversizedImage{}, env)
	assertFindings(t, got, 1)

	// The recommendation should be tailored, not generic advice.
	nodeEnv := &model.Environment{Images: []model.Image{
		{ID: "sha256:n", RepoTags: []string{"node:20"}, Size: 2_000_000_000},
	}}
	nodeFinding := run(t, OversizedImage{}, nodeEnv)[0]
	if !strings.Contains(nodeFinding.Recommendation, "npm ci") {
		t.Errorf("Node image advice should be Node-specific, got: %s", nodeFinding.Recommendation)
	}
}

func TestOversizedImageThresholdIsConfigurable(t *testing.T) {
	env := &model.Environment{Images: []model.Image{
		{ID: "sha256:m", RepoTags: []string{"svc:1"}, Size: 500_000_000},
	}}

	target := Target{Environment: env, Options: Options{LargeImageBytes: 400_000_000}.Normalize()}
	if got := (OversizedImage{}).Check(context.Background(), target); len(got) != 1 {
		t.Fatalf("lowered threshold: got %d findings, want 1", len(got))
	}
	assertFindings(t, run(t, OversizedImage{}, env), 0)
}

// --- DD017 / DD018 ----------------------------------------------------------

func TestUnusedVolume(t *testing.T) {
	env := &model.Environment{Volumes: []model.Volume{
		{Name: "used", InUse: true, Mountpoint: "/m/used"},
		{Name: "orphan", Mountpoint: "/m/orphan"},
		{Name: "156aa8105ebc31baa8874d56fd5eb1fc70999e8c7114d352eb129d0b91782569", Mountpoint: "/m/anon"},
	}}

	got := run(t, UnusedVolume{}, env)
	assertFindings(t, got, 2)

	// Every recommendation must warn before it suggests deleting data.
	for _, f := range got {
		if !strings.Contains(f.Recommendation, "Check the contents") {
			t.Errorf("volume advice must not suggest deletion without a check: %s", f.Recommendation)
		}
	}
}

func TestUnusedNetwork(t *testing.T) {
	env := &model.Environment{Networks: []model.Network{
		{ID: "1", Name: "bridge"},
		{ID: "2", Name: "host"},
		{ID: "3", Name: "none"},
		{ID: "4", Name: "app_default", Containers: []string{"api"}},
		{ID: "5", Name: "orphan_default"},
	}}

	got := run(t, UnusedNetwork{}, env)
	assertFindings(t, got, 1)
	if got[0].ResourceName != "orphan_default" {
		t.Errorf("flagged %q; Docker's predefined networks must never be reported",
			got[0].ResourceName)
	}
}

// --- helpers ----------------------------------------------------------------

func TestNormalizeHostPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/host_mnt/Users/me/code", "/Users/me/code"},
		{"/host_mnt", "/"},
		{"/run/desktop/mnt/host/c/code", "/c/code"},
		{"/var/lib/docker/", "/var/lib/docker"},
		{"/etc/./passwd", "/etc/passwd"},
		{"/plain/path", "/plain/path"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeHostPath(tt.in); got != tt.want {
			t.Errorf("normalizeHostPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOptionsNormalize(t *testing.T) {
	got := Options{}.Normalize()
	want := DefaultOptions()
	if got != want {
		t.Errorf("zero Options normalized to %+v, want %+v", got, want)
	}

	partial := Options{LargeImageBytes: 42}.Normalize()
	if partial.LargeImageBytes != 42 {
		t.Error("Normalize overwrote an explicitly set value")
	}
	if partial.RestartLoopThreshold != want.RestartLoopThreshold {
		t.Error("Normalize did not fill the unset field")
	}
}

// TestEveryRuleHasAnExplanation is the drift guard.
//
// Explanations live in a table rather than on the Rule interface, so that
// adding a rule stays a two-line change. The cost of that choice is that the
// table can fall behind the registry — this is what stops it.
func TestEveryRuleHasAnExplanation(t *testing.T) {
	for _, r := range All() {
		e, ok := Explain(r.ID())
		if !ok {
			t.Errorf("%s (%s) has no explanation — add one to internal/rules/explain.go",
				r.ID(), r.Name())
			continue
		}

		if strings.TrimSpace(e.What) == "" {
			t.Errorf("%s: What is empty", r.ID())
		}
		if strings.TrimSpace(e.Why) == "" {
			t.Errorf("%s: Why is empty", r.ID())
		}

		// An explanation without a fix is a complaint. Every rule that a user
		// can act on has to say how.
		if len(e.Fixes) == 0 {
			t.Errorf("%s: no fixes — an explanation with no remedy is not actionable", r.ID())
		}
		for i, fix := range e.Fixes {
			if strings.TrimSpace(fix.Title) == "" || strings.TrimSpace(fix.Code) == "" {
				t.Errorf("%s: fix %d is incomplete", r.ID(), i)
			}
			switch fix.Lang {
			case "bash", "dockerfile", "yaml":
			default:
				t.Errorf("%s: fix %d has unknown language %q", r.ID(), i, fix.Lang)
			}
		}

		for _, ref := range e.References {
			if !strings.HasPrefix(ref.URL, "https://") {
				t.Errorf("%s: reference %q is not an https URL", r.ID(), ref.Title)
			}
		}
	}
}

// TestExplanationsMatchRegisteredRules catches the other direction: an
// explanation left behind after its rule was removed.
func TestExplanationsMatchRegisteredRules(t *testing.T) {
	for id := range explanations {
		if _, ok := ByID(id); !ok {
			t.Errorf("explanation for %s has no matching rule", id)
		}
	}
}

func TestExplainUnknownRule(t *testing.T) {
	if _, ok := Explain("DD999"); ok {
		t.Error("an unknown rule should not resolve to an explanation")
	}
}
