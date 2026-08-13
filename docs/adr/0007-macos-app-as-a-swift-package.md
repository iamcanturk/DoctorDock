# ADR-0007: The macOS app is a Swift package, not an Xcode project

- **Status:** accepted
- **Date:** 2026-08-13

## Context

[ADR-0004](0004-json-as-the-gui-contract.md) settled that the macOS app is
native SwiftUI and talks to the Go core over JSON. That left how it is built.

The default for a macOS app is an `.xcodeproj`. That has two costs for an
open-source project: the file is a large generated XML blob that produces
unreadable diffs and constant merge conflicts, and building it requires Xcode —
a 15 GB download — even for someone who only wants to fix a typo.

## Decision

The app is a Swift package (`app/macos/Package.swift`) with a shell script that
assembles the `.app` bundle around the executable SPM produces.

`swift build` works with only the Command Line Tools, which was verified before
committing to this: SwiftUI, AppKit and `MenuBarExtra` are all in the SDK that
ships with them. Nobody needs Xcode to build DoctorDock. Anyone who has it can
still open `Package.swift` in it and get a normal editing experience.

`scripts/build-app.sh` does what Xcode would: writes `Info.plist` with the
version substituted in, lays out `Contents/MacOS` and `Contents/Resources`,
copies the `doctordock` engine into the bundle, and ad-hoc signs the result.

## Rationale

- **No Xcode requirement.** This is the whole point. The barrier to
  contributing is `git clone` and `make app`.
- **No generated project file in git.** The build is a 60-line script that can
  be read and reviewed.
- **The same build works everywhere.** There is no Xcode-version-specific
  project format to break.

## Consequences

- The bundle layout is hand-written. If it drifts from what macOS expects, that
  is our bug rather than Xcode's. `Info.plist` is small and stable enough for
  this to be a fair trade.
- No storyboards, xibs or asset catalogs — everything is code. For an app whose
  entire UI is SwiftUI, that costs nothing.
- **Notarization is not solved by this.** Ad-hoc signing is enough to run the
  app on the machine that built it. Shipping it to other people needs a
  Developer ID certificate and a notarization step, which is an Apple account
  question rather than a build-system one. Until then the install path is
  `make app-install`.
- The app is **not sandboxed**. It runs the `doctordock` binary, which talks to
  the Docker socket; the sandbox would block that, and a developer tool
  distributed outside the App Store gains nothing from it.

## The self-test

`DoctorDock --selftest` runs a scan through the same code path the UI uses,
decodes it, and asserts the results, then exits without creating a window.

This exists because the failures that matter in this app are not visual. They
are a renamed JSON field, a timestamp format the decoder rejects, a field that
is optional on somebody else's daemon — all of which produce an app that
launches happily and shows nothing. A screenshot cannot catch any of them; a
scripted check catches all of them, and runs as part of `make verify`.
