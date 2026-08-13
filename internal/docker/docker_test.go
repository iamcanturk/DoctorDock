package docker

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// TestEnvKeysDiscardsValues is the test that enforces
// docs/adr/0005-no-secret-collection.md. If it ever fails, secrets are
// reaching reports that land in CI logs.
func TestEnvKeysDiscardsValues(t *testing.T) {
	got := envKeys([]string{
		"MYSQL_ROOT_PASSWORD=hunter2",
		"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG",
		"DATABASE_URL=postgres://user:pass@host/db",
		"PATH=/usr/bin",
		"BARE_KEY",
		"",
	})

	want := []string{"AWS_SECRET_ACCESS_KEY", "BARE_KEY", "DATABASE_URL", "MYSQL_ROOT_PASSWORD", "PATH"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (sorted)", got, want)
		}
	}

	for _, key := range got {
		for _, secret := range []string{"hunter2", "wJalrXUtnFEMI", "pass@host"} {
			if contains(key, secret) {
				t.Fatalf("environment value %q leaked into key %q", secret, key)
			}
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestHasHealthcheck(t *testing.T) {
	tests := []struct {
		name string
		hc   *container.HealthConfig
		want bool
	}{
		{"nil", nil, false},
		{"empty test", &container.HealthConfig{}, false},
		// Docker's way of disabling a healthcheck inherited from the image.
		{"explicitly none", &container.HealthConfig{Test: []string{"NONE"}}, false},
		{"lower-case none", &container.HealthConfig{Test: []string{"none"}}, false},
		{"cmd", &container.HealthConfig{Test: []string{"CMD", "curl", "-f", "http://localhost"}}, true},
		{"cmd-shell", &container.HealthConfig{Test: []string{"CMD-SHELL", "pg_isready"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasHealthcheck(tt.hc); got != tt.want {
				t.Errorf("hasHealthcheck = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNormalizeCaps matters because Docker accepts three spellings of the same
// capability and rules compare against exactly one.
func TestNormalizeCaps(t *testing.T) {
	got := normalizeCaps([]string{"CAP_SYS_ADMIN", "sys_ptrace", " NET_ADMIN ", ""})
	want := []string{"NET_ADMIN", "SYS_ADMIN", "SYS_PTRACE"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestConvertPortsCollapsesDualStack covers what a real daemon does: one
// `-p 3307:3306` is reported once for 0.0.0.0 and once for ::.
func TestConvertPortsCollapsesDualStack(t *testing.T) {
	got := convertPorts([]container.Port{
		{PrivatePort: 3306, PublicPort: 3307, Type: "tcp", IP: "0.0.0.0"},
		{PrivatePort: 3306, PublicPort: 3307, Type: "tcp", IP: "::"},
	})

	if len(got) != 1 {
		t.Fatalf("dual-stack publish produced %d ports, want 1: %+v", len(got), got)
	}
	if got[0].HostIP != "0.0.0.0" {
		t.Errorf("HostIP = %q, want the IPv4 spelling users recognise", got[0].HostIP)
	}
}

func TestConvertPortsKeepsDistinctBindings(t *testing.T) {
	got := convertPorts([]container.Port{
		{PrivatePort: 5432, PublicPort: 5432, Type: "tcp", IP: "127.0.0.1"},
		{PrivatePort: 5432, PublicPort: 5433, Type: "tcp", IP: "0.0.0.0"},
		{PrivatePort: 80, Type: "tcp"},
	})

	if len(got) != 3 {
		t.Fatalf("got %d ports, want 3: %+v", len(got), got)
	}
	// Sorted by private port.
	if got[0].PrivatePort != 80 {
		t.Errorf("ports are not sorted: %+v", got)
	}
	// A loopback binding must stay distinguishable from a wildcard one, or
	// DD006 would fire on a correctly-secured database.
	var sawLoopback bool
	for _, p := range got {
		if p.HostIP == "127.0.0.1" && !p.IsPublicallyBound() {
			sawLoopback = true
		}
	}
	if !sawLoopback {
		t.Error("the loopback binding was collapsed into the wildcard one")
	}
}

func TestParseDockerTime(t *testing.T) {
	got := parseDockerTime("2026-08-10T06:37:16.255850125Z")
	want := time.Date(2026, 8, 10, 6, 37, 16, 255850125, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Docker uses the zero time to mean "never".
	for _, in := range []string{"", "0001-01-01T00:00:00Z", "not a time"} {
		if !parseDockerTime(in).IsZero() {
			t.Errorf("parseDockerTime(%q) should be the zero time", in)
		}
	}
}

func TestParseSecurityOptions(t *testing.T) {
	got := parseSecurityOptions([]string{
		"name=seccomp,profile=builtin",
		"name=cgroupns",
		"name=rootless",
		"apparmor",
	})
	want := []string{"seccomp", "cgroupns", "rootless", "apparmor"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestCleanRefs(t *testing.T) {
	got := cleanRefs([]string{"nginx:1.25", "<none>:<none>", "", "<none>@<none>", "app:latest"})
	if len(got) != 2 || got[0] != "app:latest" || got[1] != "nginx:1.25" {
		t.Errorf("got %v, want the two real references sorted", got)
	}
}

func TestContainerName(t *testing.T) {
	if got := containerName(container.Summary{Names: []string{"/api"}}); got != "api" {
		t.Errorf("got %q, want api", got)
	}
	// An unnamed container falls back to its short ID.
	got := containerName(container.Summary{ID: "abcdef0123456789abcdef"})
	if got != "abcdef012345" {
		t.Errorf("got %q, want the 12-character short ID", got)
	}
}

func TestConnectionErrorHint(t *testing.T) {
	err := &ConnectionError{Host: "unix:///var/run/docker.sock"}
	if err.Hint() == "" {
		t.Error("a connection error should carry actionable advice")
	}
	if !contains(err.Error(), "unix:///var/run/docker.sock") {
		t.Errorf("the error should name the endpoint: %v", err)
	}
}

// TestFakeSatisfiesClient keeps the test double honest when the interface
// changes.
func TestFakeSatisfiesClient(t *testing.T) {
	var c Client = &Fake{
		Containers: []model.Container{{Name: "api"}},
	}

	ctx := context.Background()
	if err := c.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := c.ListContainers(ctx)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListContainers = %v, %v", got, err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestForEachRunsEveryIndex(t *testing.T) {
	for _, n := range []int{0, 1, 2, 50} {
		seen := make([]bool, n)
		forEach(n, func(i int) { seen[i] = true })
		for i, ok := range seen {
			if !ok {
				t.Errorf("n=%d: index %d was skipped", n, i)
			}
		}
	}
}

// TestDanglingMatchesDockersDefinition pins DD014's meaning to what
// `docker image prune` actually removes. An image that kept a digest reference
// after losing its tag is not dangling: prune leaves it, so reporting it would
// send the user after a command that does nothing.
func TestDanglingMatchesDockersDefinition(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		digests  []string
		dangling bool
	}{
		{"tagged", []string{"nginx:1.25"}, []string{"nginx@sha256:abc"}, false},
		{"tag only", []string{"local/app:v1"}, nil, false},
		{"digest only, no tag", nil, []string{"nginx@sha256:abc"}, false},
		{"no references at all", nil, nil, true},
		{"placeholder references", []string{"<none>:<none>"}, []string{"<none>@<none>"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags, digests := cleanRefs(tt.tags), cleanRefs(tt.digests)
			got := len(tags) == 0 && len(digests) == 0
			if got != tt.dangling {
				t.Errorf("dangling = %v, want %v (tags=%v digests=%v)",
					got, tt.dangling, tags, digests)
			}
		})
	}
}
