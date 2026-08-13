// Package docker is the only place in DoctorDock that knows the Docker SDK
// exists.
//
// Everything it returns is a pkg/model type. SDK types never escape this
// package, so an SDK upgrade touches one package rather than the whole rule
// set. See docs/adr/0002-docker-sdk-over-raw-http.md.
package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/docker/docker/client"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Client is the surface DoctorDock needs from a Docker daemon.
//
// It exists so that unit tests can run without a daemon — CI covers three
// operating systems and requiring a live daemon on all of them is fragile. See
// Fake for the test implementation.
type Client interface {
	// Ping verifies the daemon is reachable. It is cheaper than Info and is
	// used for connectivity checks.
	Ping(ctx context.Context) error

	// Info describes the daemon.
	Info(ctx context.Context) (model.DockerInfo, error)

	// ListContainers returns every container, running or not.
	ListContainers(ctx context.Context) ([]model.Container, error)

	// ListImages returns every top-level image, including dangling ones.
	ListImages(ctx context.Context) ([]model.Image, error)

	// ListVolumes returns every volume.
	ListVolumes(ctx context.Context) ([]model.Volume, error)

	// ListNetworks returns every network. Container attachments are not
	// populated here — the scanner resolves them from the container list,
	// because the daemon's network list endpoint does not report them.
	ListNetworks(ctx context.Context) ([]model.Network, error)

	// Close releases the underlying connection.
	Close() error
}

// ConnectionError reports that the Docker daemon could not be reached, and
// carries a platform-appropriate hint for the user.
type ConnectionError struct {
	Host string
	Err  error
}

func (e *ConnectionError) Error() string {
	if e.Host != "" {
		return fmt.Sprintf("cannot reach the Docker daemon at %s: %v", e.Host, e.Err)
	}
	return fmt.Sprintf("cannot reach the Docker daemon: %v", e.Err)
}

func (e *ConnectionError) Unwrap() error { return e.Err }

// Hint returns actionable advice for a user whose daemon is unreachable.
//
// The advice is specific to how DoctorDock is running, because the fixes have
// nothing in common: a developer on a Mac needs Docker Desktop started, while
// the same failure inside a container almost always means the socket is not
// mounted or is not readable by the non-root user the image runs as.
func (e *ConnectionError) Hint() string {
	if hint, ok := containerHint(); ok {
		return hint
	}

	switch runtime.GOOS {
	case "darwin":
		return "Is Docker Desktop (or Colima/OrbStack/Rancher Desktop) running?\n" +
			"If you use a non-default context, try: DOCKER_HOST=$(docker context inspect -f '{{.Endpoints.docker.Host}}') doctordock"
	case "windows":
		return "Is Docker Desktop running?\n" +
			"If you use a non-default context, set DOCKER_HOST to its endpoint."
	default:
		return "Is the Docker daemon running (systemctl status docker)?\n" +
			"If your user is not in the 'docker' group you may need sudo, or set DOCKER_HOST."
	}
}

// engineClient is the production Client, backed by the Docker Engine API.
type engineClient struct {
	api  *client.Client
	host string

	// imageUserCache memoizes the image-configured user across containers, so
	// that resolving the effective user costs one inspect per distinct image
	// rather than one per container.
	imageUserMu    sync.Mutex
	imageUserCache map[string]string
}

// Connect opens a connection to the Docker daemon.
//
// Connection details come from the environment (DOCKER_HOST, DOCKER_CONTEXT,
// DOCKER_TLS_VERIFY, DOCKER_CERT_PATH), so Colima, OrbStack, Rancher Desktop,
// Podman's Docker-compatible socket and remote daemons all work without any
// DoctorDock-specific configuration.
//
// The API version is negotiated down to whatever the daemon supports, so old
// daemons do not produce cryptic version errors.
func Connect(ctx context.Context) (Client, error) {
	api, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}

	c := &engineClient{
		api:            api,
		host:           api.DaemonHost(),
		imageUserCache: make(map[string]string),
	}

	if err := c.Ping(ctx); err != nil {
		_ = api.Close()
		var connErr *ConnectionError
		if errors.As(err, &connErr) {
			return nil, err
		}
		return nil, &ConnectionError{Host: c.host, Err: err}
	}

	return c, nil
}

// Host returns the daemon endpoint this client is connected to.
func (c *engineClient) Host() string { return c.host }

func (c *engineClient) Ping(ctx context.Context) error {
	if _, err := c.api.Ping(ctx); err != nil {
		return &ConnectionError{Host: c.host, Err: err}
	}
	return nil
}

func (c *engineClient) Close() error {
	if c.api == nil {
		return nil
	}
	return c.api.Close()
}

// forEach runs fn over indexes [0, n) with bounded concurrency.
//
// Collection issues one inspect call per container and per image. Those are
// local socket round-trips: individually fast, but serialized across a busy
// machine they add up to a noticeable pause. A small fixed pool removes that
// without risking a burst of hundreds of concurrent requests at the daemon.
func forEach(n int, fn func(i int)) {
	const workers = 8

	if n <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}(i)
	}
	wg.Wait()
}

// defaultSocketPath is where the Docker socket lives on Linux, which is the
// only platform the container image runs on.
const defaultSocketPath = "/var/run/docker.sock"

// containerHint returns advice tailored to running inside a container, and
// false when DoctorDock is not containerized.
func containerHint() (string, bool) {
	if !inContainer() {
		return "", false
	}

	if _, err := os.Stat(defaultSocketPath); err != nil {
		return "DoctorDock is running inside a container, but the Docker socket is not mounted.\n" +
			"Mount it read-only:\n" +
			"  docker run --rm -v " + defaultSocketPath + ":" + defaultSocketPath + ":ro \\\n" +
			"    ghcr.io/iamcanturk/doctordock", true
	}

	// The socket exists but could not be used. The image runs as a non-root
	// user by design, so the usual cause is that the socket's group is not one
	// of ours.
	return "DoctorDock is running inside a container as a non-root user and cannot read the\n" +
		"Docker socket. Grant it the socket's group:\n" +
		"  docker run --rm --group-add \"$(stat -c '%g' " + defaultSocketPath + ")\" \\\n" +
		"    -v " + defaultSocketPath + ":" + defaultSocketPath + ":ro \\\n" +
		"    ghcr.io/iamcanturk/doctordock\n" +
		"On Docker Desktop the socket is owned by root, so use --group-add 0.", true
}

// inContainer reports whether this process is running inside a container.
//
// /.dockerenv is created by the Docker daemon in every container it starts.
// The check is best-effort: a false negative only costs a less specific hint.
func inContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}
