// Package scanner orchestrates a scan: collect the environment, resolve
// relationships between resources, run the rules, and assemble a report.
package scanner

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/internal/score"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Config controls a scan.
type Config struct {
	// IgnoreRules lists rule IDs to skip. They are recorded in the report so
	// that a reader can tell "clean" apart from "not checked".
	IgnoreRules []string

	// Options carries tunable rule thresholds. Zero values take their defaults.
	Options rules.Options

	// Scorer computes the health score. Nil means score.Default().
	Scorer score.Scorer

	// Rules overrides the rule set. Nil means the full registry. This exists
	// for tests and for `doctordock security`, which runs the security subset.
	Rules []rules.Rule

	// IncludeResources controls whether the full resource lists are attached
	// to the report. The terminal renderer needs summaries only; the JSON
	// consumers want everything.
	IncludeResources bool

	// Tool identifies the binary in the report.
	Tool model.ToolInfo

	// Now overrides the report timestamp. Zero means time.Now().
	Now time.Time
}

// Scanner runs scans against one Docker client.
type Scanner struct {
	client docker.Client
	cfg    Config
}

// New returns a Scanner. The client is not owned by the Scanner; the caller
// closes it.
func New(client docker.Client, cfg Config) *Scanner {
	cfg.Options = cfg.Options.Normalize()
	if cfg.Scorer == nil {
		cfg.Scorer = score.Default()
	}
	if cfg.Rules == nil {
		cfg.Rules = rules.All()
	}
	return &Scanner{client: client, cfg: cfg}
}

// Collect gathers a snapshot of the Docker environment and resolves the
// relationships between its resources.
//
// The four list calls are independent, so they run concurrently: on a busy
// machine each involves a round of inspects, and serializing them triples the
// time to first output for no reason.
func (s *Scanner) Collect(ctx context.Context) (*model.Environment, error) {
	env := &model.Environment{CollectedAt: s.now()}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	record := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}

	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record(fn())
		}()
	}

	run(func() (err error) { env.Docker, err = s.client.Info(ctx); return })
	run(func() (err error) { env.Containers, err = s.client.ListContainers(ctx); return })
	run(func() (err error) { env.Images, err = s.client.ListImages(ctx); return })
	run(func() (err error) { env.Volumes, err = s.client.ListVolumes(ctx); return })
	run(func() (err error) { env.Networks, err = s.client.ListNetworks(ctx); return })

	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("collect docker environment: %w", errors.Join(errs...))
	}

	link(env)
	return env, nil
}

// Scan collects the environment, evaluates every enabled rule against it, and
// returns a complete report.
func (s *Scanner) Scan(ctx context.Context) (*model.Report, error) {
	env, err := s.Collect(ctx)
	if err != nil {
		return nil, err
	}
	return s.Evaluate(ctx, env)
}

// Evaluate runs the rules against an already-collected environment. It is
// separate from Scan so that a caller holding a snapshot — a test, or a future
// `--from-file` mode — can re-run the rules without touching Docker.
func (s *Scanner) Evaluate(ctx context.Context, env *model.Environment) (*model.Report, error) {
	ignored := make(map[string]bool, len(s.cfg.IgnoreRules))
	for _, id := range s.cfg.IgnoreRules {
		ignored[id] = true
	}

	target := rules.Target{Environment: env, Options: s.cfg.Options}

	var (
		findings []model.Finding
		skipped  []string
	)
	for _, rule := range s.cfg.Rules {
		if ignored[rule.ID()] {
			skipped = append(skipped, rule.ID())
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		findings = append(findings, rule.Check(ctx, target)...)
	}

	sortFindings(findings)
	sort.Strings(skipped)

	report := &model.Report{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   s.now(),
		Tool:          s.cfg.Tool,
		Docker:        env.Docker,
		Score:         s.cfg.Scorer.Calculate(findings),
		Summary:       model.Summarize(env, findings),
		Findings:      findings,
		SkippedRules:  skipped,
	}
	if report.Findings == nil {
		report.Findings = []model.Finding{}
	}

	if s.cfg.IncludeResources {
		report.Containers = env.Containers
		report.Images = env.Images
		report.Volumes = env.Volumes
		report.Networks = env.Networks
	}

	return report, nil
}

func (s *Scanner) now() time.Time {
	if !s.cfg.Now.IsZero() {
		return s.cfg.Now
	}
	return time.Now().UTC()
}

// sortFindings orders findings most severe first, then by rule ID, then by
// resource name. Deterministic ordering is what makes two scans of an
// unchanged machine diffable.
func sortFindings(findings []model.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Severity.Rank() != b.Severity.Rank() {
			return a.Severity.Rank() > b.Severity.Rank()
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.ResourceName < b.ResourceName
	})
}
