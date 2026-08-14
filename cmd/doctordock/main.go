// Command doctordock inspects the local Docker environment and reports
// security problems, misconfigurations and reclaimable resources.
//
// It runs entirely locally: no network calls, no telemetry, no AI.
package main

import (
	"os"

	"github.com/iamcanturk/DoctorDock/internal/cli"
)

// Set at build time with -ldflags. The defaults are what a `go build` with no
// flags produces, and what `go install` from a module produces.
var (
	version = "dev"
	commit  = ""
)

func main() {
	os.Exit(cli.Execute(version, commit))
}
