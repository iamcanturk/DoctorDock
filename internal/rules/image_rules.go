package rules

import (
	"context"
	"fmt"
	"strings"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// --- DD014 ------------------------------------------------------------------

// DanglingImage reports untagged leftovers from rebuilds.
type DanglingImage struct{}

func (DanglingImage) ID() string               { return "DD014" }
func (DanglingImage) Name() string             { return "Dangling image" }
func (DanglingImage) Category() model.Category { return model.CategoryCleanup }
func (DanglingImage) Severity() model.Severity { return model.SeverityLow }
func (DanglingImage) Description() string {
	return "Reports untagged images left behind when a rebuild moved a tag to a new image."
}

func (r DanglingImage) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, img := range t.Environment.Images {
		if !img.Dangling || img.InUse {
			continue
		}
		f := newImageFinding(r, img)
		f.Title = fmt.Sprintf("Dangling image %s (%s)", img.ShortID(), model.FormatBytes(img.Size))
		f.Description = "This image has no tag and no container uses it. It is what a previous " +
			"build left behind when its tag was moved to a newer image, and it will never be " +
			"referenced again."
		f.Recommendation = "Remove all dangling images with `docker image prune`."
		f.Details = map[string]string{"size_bytes": fmt.Sprintf("%d", img.Size)}
		out = append(out, f)
	}
	return out
}

// --- DD015 ------------------------------------------------------------------

// UnusedImage reports tagged images that no container references.
type UnusedImage struct{}

func (UnusedImage) ID() string               { return "DD015" }
func (UnusedImage) Name() string             { return "Unused image" }
func (UnusedImage) Category() model.Category { return model.CategoryCleanup }
func (UnusedImage) Severity() model.Severity { return model.SeverityInfo }
func (UnusedImage) Description() string {
	return "Reports tagged images that no container references, and the disk they occupy."
}

func (r UnusedImage) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, img := range t.Environment.Images {
		// Dangling images are also unused; DD014 owns them so that a single
		// leftover image is not reported twice.
		if img.InUse || img.Dangling {
			continue
		}
		f := newImageFinding(r, img)
		f.Title = fmt.Sprintf("Unused image %s (%s)", img.DisplayName(), model.FormatBytes(img.Size))
		f.Description = "No container, running or stopped, references this image. It may still be " +
			"a base image you pull often, or it may be from a project you finished months ago."
		f.Recommendation = fmt.Sprintf(
			"Remove it with `docker rmi %s` if you no longer need it, or clear every unused image "+
				"at once with `docker image prune -a`.", img.DisplayName())
		f.Details = map[string]string{"size_bytes": fmt.Sprintf("%d", img.Size)}
		out = append(out, f)
	}
	return out
}

// --- DD016 ------------------------------------------------------------------

// OversizedImage reports images large enough to slow down every pull and
// deploy that touches them.
type OversizedImage struct{}

func (OversizedImage) ID() string               { return "DD016" }
func (OversizedImage) Name() string             { return "Oversized image" }
func (OversizedImage) Category() model.Category { return model.CategoryResource }
func (OversizedImage) Severity() model.Severity { return model.SeverityLow }
func (OversizedImage) Description() string {
	return "Reports images above the configured size threshold (1.5 GB by default)."
}

func (r OversizedImage) Check(_ context.Context, t Target) []model.Finding {
	var out []model.Finding
	for _, img := range t.Environment.Images {
		if img.Size < t.Options.LargeImageBytes {
			continue
		}
		f := newImageFinding(r, img)
		f.Title = fmt.Sprintf("Image %s is %s", img.DisplayName(), model.FormatBytes(img.Size))
		f.Description = fmt.Sprintf(
			"At %s this image is above the %s threshold. Size is paid on every pull, every cold "+
				"start and every registry push, and a larger base image also means a larger "+
				"package surface to keep patched.",
			model.FormatBytes(img.Size), model.FormatBytes(t.Options.LargeImageBytes))
		f.Recommendation = imageSizeAdvice(img)
		f.Details = map[string]string{
			"size_bytes": fmt.Sprintf("%d", img.Size),
			"layers":     fmt.Sprintf("%d", img.Layers),
		}
		out = append(out, f)
	}
	return out
}

// imageSizeAdvice tailors the recommendation to what the image looks like,
// because "make it smaller" on its own is not actionable.
func imageSizeAdvice(img model.Image) string {
	base := "Use a multi-stage build so that compilers, package caches and dev dependencies stay " +
		"out of the final image, and switch the runtime stage to a slim or alpine base."

	name := strings.ToLower(img.DisplayName())
	switch {
	case strings.Contains(name, "node"):
		return base + " For Node, copy only package.json and the lockfile before installing, run " +
			"`npm ci --omit=dev` in the final stage, and add a .dockerignore covering node_modules."
	case strings.Contains(name, "python"):
		return base + " For Python, prefer python:*-slim, install with `--no-cache-dir`, and build " +
			"wheels in an earlier stage."
	case strings.Contains(name, "golang") || strings.Contains(name, "go:"):
		return "Compile in a golang stage and copy only the binary into scratch, distroless or " +
			"alpine. A Go service image should be tens of megabytes, not gigabytes."
	default:
		return base + fmt.Sprintf(" `docker history %s` shows which layers are responsible.",
			img.DisplayName())
	}
}
