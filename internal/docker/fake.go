package docker

import (
	"context"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Fake is an in-memory Client for tests.
//
// It lives in the non-test build so that other packages' tests can use it
// without importing a _test file. Populate the fields directly, or set the
// Err* fields to exercise failure paths.
type Fake struct {
	DockerInfo model.DockerInfo
	Containers []model.Container
	Images     []model.Image
	Volumes    []model.Volume
	Networks   []model.Network

	PingErr       error
	InfoErr       error
	ContainersErr error
	ImagesErr     error
	VolumesErr    error
	NetworksErr   error

	// Closed records whether Close was called, so tests can assert cleanup.
	Closed bool
}

var _ Client = (*Fake)(nil)

func (f *Fake) Ping(context.Context) error { return f.PingErr }

func (f *Fake) Info(context.Context) (model.DockerInfo, error) {
	if f.InfoErr != nil {
		return model.DockerInfo{}, f.InfoErr
	}
	return f.DockerInfo, nil
}

func (f *Fake) ListContainers(context.Context) ([]model.Container, error) {
	if f.ContainersErr != nil {
		return nil, f.ContainersErr
	}
	return f.Containers, nil
}

func (f *Fake) ListImages(context.Context) ([]model.Image, error) {
	if f.ImagesErr != nil {
		return nil, f.ImagesErr
	}
	return f.Images, nil
}

func (f *Fake) ListVolumes(context.Context) ([]model.Volume, error) {
	if f.VolumesErr != nil {
		return nil, f.VolumesErr
	}
	return f.Volumes, nil
}

func (f *Fake) ListNetworks(context.Context) ([]model.Network, error) {
	if f.NetworksErr != nil {
		return nil, f.NetworksErr
	}
	return f.Networks, nil
}

func (f *Fake) Close() error {
	f.Closed = true
	return nil
}
