package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func sampleReport() *model.Report {
	findings := []model.Finding{
		{
			ID: "DD005", Rule: "Docker socket exposed",
			Severity: model.SeverityCritical, Category: model.CategorySecurity,
			Resource: model.ResourceContainer, ResourceID: "c1", ResourceName: "api",
			Title:          "Docker socket is mounted into the container",
			Description:    "Equivalent to root on the host.",
			Recommendation: "Remove the socket mount.",
		},
	}
	for _, name := range []string{"api", "db", "cache", "worker", "web", "queue", "cron"} {
		findings = append(findings, model.Finding{
			ID: "DD001", Rule: "Container runs as root",
			Severity: model.SeverityHigh, Category: model.CategorySecurity,
			Resource: model.ResourceContainer, ResourceID: name, ResourceName: name,
			Title:          "Container runs as root",
			Description:    "Processes start as uid 0.",
			Recommendation: "Add a non-root USER to the image.",
		})
	}

	env := &model.Environment{
		Containers: []model.Container{
			{ID: "c1", Name: "api", Image: "api:latest", State: model.StateRunning, Health: model.HealthNone,
				Ports: []model.Port{{PrivatePort: 8080, PublicPort: 8080, Type: "tcp", HostIP: "0.0.0.0"}}},
			{ID: "c2", Name: "db", Image: "postgres:16", State: model.StateExited, Health: model.HealthUnhealthy},
		},
		Images: []model.Image{
			{ID: "sha256:aaaaaaaaaaaaaaaa", RepoTags: []string{"api:latest"}, Size: 100 << 20, InUse: true, UsedBy: []string{"api"}},
			{ID: "sha256:bbbbbbbbbbbbbbbb", Size: 50 << 20, Dangling: true},
		},
		Volumes:  []model.Volume{{Name: "data", Driver: "local", InUse: true, UsedBy: []string{"db"}}},
		Networks: []model.Network{{ID: "n1", Name: "app_net", Driver: "bridge", Scope: "local", Subnets: []string{"172.20.0.0/16"}}},
	}

	return &model.Report{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		Tool:          model.ToolInfo{Name: "doctordock", Version: "0.1.0"},
		Docker:        model.DockerInfo{ServerVersion: "28.0.0", OSType: "linux", Architecture: "arm64"},
		Score:         42,
		Summary:       model.Summarize(env, findings),
		Findings:      findings,
		Containers:    env.Containers,
		Images:        env.Images,
		Volumes:       env.Volumes,
		Networks:      env.Networks,
	}
}

func render(t *testing.T, r Renderer, rep *model.Report) string {
	t.Helper()
	var buf bytes.Buffer
	if err := r.Render(&buf, rep); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestParseFormat(t *testing.T) {
	for _, in := range []string{"", "terminal", "TEXT", " tty "} {
		if got, err := ParseFormat(in); err != nil || got != FormatTerminal {
			t.Errorf("ParseFormat(%q) = %q, %v", in, got, err)
		}
	}
	if got, err := ParseFormat("JSON"); err != nil || got != FormatJSON {
		t.Errorf("ParseFormat(JSON) = %q, %v", got, err)
	}
	if _, err := ParseFormat("yaml"); err == nil {
		t.Error("unknown format should be rejected")
	}
}

func TestJSONRendererRoundTrips(t *testing.T) {
	out := render(t, NewJSON(), sampleReport())

	var decoded model.Report
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Score != 42 || decoded.SchemaVersion != model.SchemaVersion {
		t.Errorf("round trip lost data: %+v", decoded)
	}
	if len(decoded.Findings) != len(sampleReport().Findings) {
		t.Error("round trip lost findings")
	}
}

// TestJSONDoesNotEscapeHTML matters because recommendations contain shell
// snippets and image references with characters json would otherwise mangle.
func TestJSONDoesNotEscapeHTML(t *testing.T) {
	rep := sampleReport()
	rep.Findings[0].Recommendation = "Use `-p 127.0.0.1:5432:5432` & drop <none> tags"

	out := render(t, NewJSON(), rep)
	if strings.Contains(out, `\u0026`) || strings.Contains(out, `\u003c`) {
		t.Errorf("output contains HTML escapes:\n%s", out)
	}
}

func TestTerminalRendererGroupsRepeats(t *testing.T) {
	out := render(t, NewTerminal(Options{}), sampleReport())

	// Seven identical DD001 findings must collapse into one block.
	if n := strings.Count(out, "DD001"); n != 1 {
		t.Errorf("DD001 appears %d times, want 1 grouped entry:\n%s", n, out)
	}
	if !strings.Contains(out, "7 containers") {
		t.Errorf("grouped entry should state how many resources it covers:\n%s", out)
	}
	// The names still have to appear somewhere.
	if !strings.Contains(out, "api") || !strings.Contains(out, "and 2 more") {
		t.Errorf("grouped entry should list resources and summarize the tail:\n%s", out)
	}
}

func TestTerminalRendererShowAllExpandsGroups(t *testing.T) {
	out := render(t, NewTerminal(Options{ShowAll: true}), sampleReport())

	if n := strings.Count(out, "DD001"); n != 7 {
		t.Errorf("--all should print one entry per resource, got %d:\n%s", n, out)
	}
}

func TestTerminalRendererShowsKeyNumbers(t *testing.T) {
	out := render(t, NewTerminal(Options{}), sampleReport())

	for _, want := range []string{"42/100", "poor", "CONTAINERS", "IMAGES", "VOLUMES", "NETWORKS",
		"FINDINGS", "WHAT TO FIX FIRST", "Docker 28.0.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

func TestTerminalRendererCleanEnvironment(t *testing.T) {
	rep := sampleReport()
	rep.Findings = nil
	rep.Score = 100

	out := render(t, NewTerminal(Options{}), rep)
	if !strings.Contains(out, "No findings") {
		t.Errorf("a clean environment should say so:\n%s", out)
	}
	if strings.Contains(out, "WHAT TO FIX FIRST") {
		t.Error("a clean environment should not print a fix list")
	}
}

// TestNoColorByDefault guards the piped-output case: escape sequences in a
// file or in `jq` input are a bug.
func TestNoColorByDefault(t *testing.T) {
	out := render(t, NewTerminal(Options{Color: false}), sampleReport())
	if strings.Contains(out, "\033[") {
		t.Errorf("colour disabled but output contains ANSI escapes:\n%q", out)
	}
}

func TestColorWhenEnabled(t *testing.T) {
	out := render(t, NewTerminal(Options{Color: true}), sampleReport())
	if !strings.Contains(out, "\033[") {
		t.Error("colour enabled but no ANSI escapes were emitted")
	}
}

func TestResourceRenderers(t *testing.T) {
	tests := []struct {
		kind model.ResourceKind
		want []string
	}{
		{model.ResourceContainer, []string{"NAME", "IMAGE", "STATE", "api", "postgres:16", "8080→8080/tcp"}},
		{model.ResourceImage, []string{"REPOSITORY:TAG", "SIZE", "api:latest", "USED BY"}},
		{model.ResourceVolume, []string{"NAME", "DRIVER", "data", "never deletes"}},
		{model.ResourceNetwork, []string{"NAME", "SUBNET", "app_net", "172.20.0.0/16"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			out := render(t, NewResource(tt.kind, Options{}), sampleReport())
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("%s view is missing %q:\n%s", tt.kind, want, out)
				}
			}
		})
	}
}

func TestResourceRendererHandlesEmptyEnvironment(t *testing.T) {
	rep := &model.Report{SchemaVersion: model.SchemaVersion, Findings: []model.Finding{}}
	for _, kind := range []model.ResourceKind{
		model.ResourceContainer, model.ResourceImage, model.ResourceVolume, model.ResourceNetwork,
	} {
		out := render(t, NewResource(kind, Options{}), rep)
		if !strings.Contains(out, "No ") {
			t.Errorf("%s view should state that nothing was found:\n%s", kind, out)
		}
	}
}

func TestWrap(t *testing.T) {
	got := Wrap("the quick brown fox jumps over the lazy dog", 20)
	for _, line := range got {
		if len(line) > 20 {
			t.Errorf("line exceeds width: %q", line)
		}
	}
	if strings.Join(got, " ") != "the quick brown fox jumps over the lazy dog" {
		t.Errorf("wrapping lost or reordered words: %v", got)
	}

	if Wrap("", 20) != nil {
		t.Error("empty input should wrap to nothing")
	}
	// A word longer than the width must not be dropped or truncated.
	long := Wrap("supercalifragilisticexpialidocious", 10)
	if len(long) != 1 || long[0] != "supercalifragilisticexpialidocious" {
		t.Errorf("over-long word was mangled: %v", long)
	}
}

func TestSummarizeResources(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e", "f", "g"}
	if got := summarizeResources(names, 5); got != "a, b, c, d, e and 2 more" {
		t.Errorf("got %q", got)
	}
	if got := summarizeResources([]string{"a", "b"}, 5); got != "a, b" {
		t.Errorf("got %q", got)
	}
}

func TestShouldColorRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if ShouldColor(&bytes.Buffer{}) {
		t.Error("NO_COLOR must disable colour")
	}
}

func TestShouldColorOnNonTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	if ShouldColor(&bytes.Buffer{}) {
		t.Error("a non-file writer is never a terminal")
	}
}

func TestRenderExplanationLongForm(t *testing.T) {
	detail := RuleDetail{
		ID:          "DD005",
		Name:        "Docker socket exposed",
		Severity:    model.SeverityCritical,
		Category:    model.CategorySecurity,
		Description: "short form",
		HasLongForm: true,
		Explanation: rules.Explanation{
			What:     "The Docker socket is mounted.",
			Why:      "It is equivalent to root on the host.",
			Scenario: "An attacker starts a privileged container.",
			Fixes: []rules.Fix{
				{Title: "Remove the mount", Lang: "bash", Code: "docker run myimage"},
			},
			FalsePositives: "Portainer needs it.",
			References:     []rules.Reference{{Title: "Docker docs", URL: "https://docs.docker.com/"}},
		},
	}

	var buf bytes.Buffer
	if err := RenderExplanation(&buf, detail, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"DD005", "Docker socket exposed", "CRITICAL", "security",
		"WHAT IT LOOKS FOR", "WHY IT MATTERS", "WHAT GOES WRONG",
		"HOW TO FIX IT", "Remove the mount", "docker run myimage",
		"WHEN THIS IS FINE TO IGNORE", "Portainer needs it.",
		// The suppression command has to be there: somebody reading this is
		// often deciding whether to fix it or silence it.
		"SUPPRESSING IT", "--ignore DD005",
		"FURTHER READING", "https://docs.docker.com/",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explanation output is missing %q", want)
		}
	}
}

// TestRenderExplanationFallsBackWithoutLongForm keeps a rule with no
// explanation from producing an empty page.
func TestRenderExplanationFallsBackWithoutLongForm(t *testing.T) {
	var buf bytes.Buffer
	err := RenderExplanation(&buf, RuleDetail{
		ID:          "DD099",
		Name:        "Some rule",
		Severity:    model.SeverityLow,
		Category:    model.CategoryCleanup,
		Description: "The one-line description.",
		HasLongForm: false,
	}, Options{})
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "The one-line description.") {
		t.Errorf("the short description should be shown as a fallback:\n%s", out)
	}
	if strings.Contains(out, "HOW TO FIX IT") {
		t.Error("sections with no content should not be printed")
	}
}

// TestDashboardPointsAtExplain is the discoverability guarantee: a user
// looking at "DD001" must be told how to find out what it means.
func TestDashboardPointsAtExplain(t *testing.T) {
	out := render(t, NewTerminal(Options{}), sampleReport())

	if !strings.Contains(out, "doctordock explain DD005") {
		t.Errorf("the dashboard should offer `explain` for the worst finding:\n%s", out)
	}
	if !strings.Contains(out, "doctordock cleanup") {
		t.Error("the dashboard should mention cleanup")
	}
}

// TestJargonIsGlossed covers the terms that mean nothing without the Docker
// glossary.
func TestJargonIsGlossed(t *testing.T) {
	out := render(t, NewTerminal(Options{}), sampleReport())

	for term, gloss := range map[string]string{
		"Dangling":  "untagged",
		"Anonymous": "unnamed",
		"Custom":    "you created these",
	} {
		if !strings.Contains(out, term) {
			t.Errorf("%q is missing from the dashboard", term)
			continue
		}
		if !strings.Contains(out, gloss) {
			t.Errorf("%q is shown without an explanation; expected %q nearby", term, gloss)
		}
	}
}
