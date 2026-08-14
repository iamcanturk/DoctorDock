#!/usr/bin/env bash
#
# Assembles DoctorDock.app.
#
# SPM produces a bare executable; macOS needs a bundle. This wraps one around
# it, drops the doctordock binary into Resources so the app works with nothing
# else installed, and ad-hoc signs the result so Gatekeeper will run it locally.
#
#   ./scripts/build-app.sh [--release] [--install]
#
set -euo pipefail

cd "$(dirname "$0")/.."

APP_NAME="DoctorDock"
BUNDLE_ID="dev.iamcanturk.doctordock"
CONFIG="debug"
INSTALL=0

for arg in "$@"; do
  case "$arg" in
    --release) CONFIG="release" ;;
    --install) INSTALL=1 ;;
    *) echo "unknown argument: $arg" >&2; exit 1 ;;
  esac
done

REPO_ROOT="$(cd ../.. && pwd)"
VERSION="$(cd "$REPO_ROOT" && git describe --tags --exact-match 2>/dev/null \
  || echo "0.1.0-dev")"
BUILD_DIR="build"
APP="$BUILD_DIR/$APP_NAME.app"

echo "==> Building the Swift executable ($CONFIG)"
swift build -c "$CONFIG"
EXECUTABLE="$(swift build -c "$CONFIG" --show-bin-path)/DoctorDockApp"

echo "==> Building the doctordock engine"
# The engine is bundled so the app works on a machine with nothing installed.
# DoctorDockCLI still prefers a copy on PATH when one is present, so a
# `brew upgrade doctordock` improves the app without rebuilding it.
ENGINE="$BUILD_DIR/doctordock"
mkdir -p "$BUILD_DIR"
(cd "$REPO_ROOT" && CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags "-s -w -X main.version=$VERSION -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
  -o "$PWD/app/macos/$ENGINE" ./cmd/doctordock)

echo "==> Assembling $APP"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

cp "$EXECUTABLE" "$APP/Contents/MacOS/$APP_NAME"
cp "$ENGINE" "$APP/Contents/Resources/doctordock"
chmod +x "$APP/Contents/Resources/doctordock"

sed -e "s/__VERSION__/$VERSION/g" \
    -e "s/__BUNDLE_ID__/$BUNDLE_ID/g" \
    -e "s/__APP_NAME__/$APP_NAME/g" \
    Resources/Info.plist > "$APP/Contents/Info.plist"

printf 'APPL????' > "$APP/Contents/PkgInfo"

if [ -f Resources/AppIcon.icns ]; then
  cp Resources/AppIcon.icns "$APP/Contents/Resources/"
fi

# The mascot is bundled so the share card can draw the logo. It is loaded at
# runtime from Bundle.main, so it must live in Contents/Resources.
if [ -f Resources/mascot.png ]; then
  cp Resources/mascot.png "$APP/Contents/Resources/"
fi

echo "==> Signing"
# Ad-hoc signature. It is enough for the app to run on the machine that built
# it and for UNUserNotificationCenter to accept the bundle. Distributing to
# other machines needs a Developer ID and notarization — see docs/DISTRIBUTION.md.
codesign --force --deep --sign - --timestamp=none "$APP" 2>/dev/null \
  || echo "    (ad-hoc signing failed; the app will still run locally)"

echo "==> Built $APP ($VERSION)"
du -sh "$APP" | awk '{print "    bundle size: "$1}'

if [ "$INSTALL" = "1" ]; then
  DEST="$HOME/Applications"
  mkdir -p "$DEST"
  rm -rf "$DEST/$APP_NAME.app"
  cp -R "$APP" "$DEST/"
  echo "==> Installed to $DEST/$APP_NAME.app"
fi
