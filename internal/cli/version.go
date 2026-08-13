package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/internal/report"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// buildCommit is set at build time via -ldflags. It is package state rather
// than a parameter because toolInfo needs it from deep inside the command tree.
var buildCommit string

type versionInfo struct {
	Version       string `json:"version"`
	Commit        string `json:"commit,omitempty"`
	SchemaVersion string `json:"schema_version"`
	Go            string `json:"go"`
	Platform      string `json:"platform"`

	Docker *model.DockerInfo `json:"docker,omitempty"`
	// DockerError explains why the daemon could not be reached, when it could
	// not be. Present only with --check-docker.
	DockerError string `json:"docker_error,omitempty"`
}

func newVersionCommand(g *globals, version, commit string) *cobra.Command {
	var checkDocker bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long: "Prints the DoctorDock version, the JSON schema version it produces, and the\n" +
			"platform it was built for.\n\n" +
			"With --check-docker it also verifies that a Docker daemon is reachable, which is\n" +
			"the quickest way to diagnose a connection problem.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			buildCommit = commit

			info := versionInfo{
				Version:       version,
				Commit:        commit,
				SchemaVersion: model.SchemaVersion,
				Go:            runtime.Version(),
				Platform:      runtime.GOOS + "/" + runtime.GOARCH,
			}

			if checkDocker {
				if d, err := probeDocker(cmd.Context(), g.timeout); err != nil {
					info.DockerError = err.Error()
				} else {
					info.Docker = &d
				}
			}

			format, err := report.ParseFormat(g.format)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if format == report.FormatJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			p := newPaletteFor(g, cmd)
			fmt.Fprintf(out, "\n  %s %s\n", p.Bold("doctordock"), info.Version)
			if info.Commit != "" {
				fmt.Fprintf(out, "  %s %s\n", p.Dim("commit "), info.Commit)
			}
			fmt.Fprintf(out, "  %s %s\n", p.Dim("schema "), info.SchemaVersion)
			fmt.Fprintf(out, "  %s %s\n", p.Dim("built  "), info.Go+" "+info.Platform)

			if checkDocker {
				fmt.Fprintln(out)
				if info.DockerError != "" {
					fmt.Fprintf(out, "  %s %s\n", p.Red("docker "), "unreachable")
					fmt.Fprintf(out, "  %s\n", p.Dim(info.DockerError))
					return &failThreshold{code: ExitError}
				}
				fmt.Fprintf(out, "  %s %s\n", p.Green("docker "),
					fmt.Sprintf("%s · %s · %s/%s",
						info.Docker.ServerVersion, info.Docker.OperatingSystem,
						info.Docker.OSType, info.Docker.Architecture))
			}

			fmt.Fprintln(out)
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkDocker, "check-docker", false, "also verify that the Docker daemon is reachable")
	return cmd
}

func probeDocker(ctx context.Context, timeout time.Duration) (model.DockerInfo, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := docker.Connect(ctx)
	if err != nil {
		return model.DockerInfo{}, err
	}
	defer client.Close()

	return client.Info(ctx)
}
