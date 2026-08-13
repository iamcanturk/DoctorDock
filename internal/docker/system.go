package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Info describes the daemon DoctorDock is connected to.
func (c *engineClient) Info(ctx context.Context) (model.DockerInfo, error) {
	info, err := c.api.Info(ctx)
	if err != nil {
		return model.DockerInfo{}, fmt.Errorf("daemon info: %w", err)
	}

	out := model.DockerInfo{
		ServerVersion:   info.ServerVersion,
		APIVersion:      c.api.ClientVersion(),
		OSType:          info.OSType,
		Architecture:    info.Architecture,
		KernelVersion:   info.KernelVersion,
		OperatingSystem: info.OperatingSystem,
		StorageDriver:   info.Driver,
		CgroupVersion:   info.CgroupVersion,
		CPUs:            info.NCPU,
		MemTotal:        info.MemTotal,
		SecurityOptions: parseSecurityOptions(info.SecurityOptions),
	}

	for _, opt := range out.SecurityOptions {
		if opt == "rootless" {
			out.Rootless = true
		}
	}

	return out, nil
}

// parseSecurityOptions turns the daemon's encoded option strings into plain
// names. The daemon reports entries like "name=seccomp,profile=builtin" and
// "name=rootless"; only the name is meaningful to us.
func parseSecurityOptions(opts []string) []string {
	out := make([]string, 0, len(opts))
	for _, opt := range opts {
		for _, field := range strings.Split(opt, ",") {
			name, value, found := strings.Cut(field, "=")
			if found && name == "name" {
				out = append(out, value)
				break
			}
			if !found && field != "" {
				out = append(out, field)
				break
			}
		}
	}
	return out
}
