#!/usr/bin/env node
// Downloads the DoctorDock binary for this platform from the matching GitHub
// release and verifies it against the release checksum file.
//
// The binary is not vendored into the package because that would mean shipping
// six binaries — roughly 100 MB — to every user so that they can use one.

import { chmod, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const here = dirname(fileURLToPath(import.meta.url));
const pkg = JSON.parse(await readFile(join(here, "package.json"), "utf8"));

const VERSION = pkg.version;
const REPO = "iamcanturk/DoctorDock";
const BIN_DIR = join(here, "bin");

const PLATFORMS = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCHS = {
  x64: "amd64",
  arm64: "arm64",
};

function target() {
  const os = PLATFORMS[process.platform];
  const arch = ARCHS[process.arch];

  if (!os || !arch) {
    throw new Error(
      `DoctorDock has no prebuilt binary for ${process.platform}/${process.arch}.\n` +
        `Build from source instead: go install github.com/${REPO}/cmd/doctordock@v${VERSION}`,
    );
  }
  return { os, arch, windows: os === "windows" };
}

async function download(url) {
  const res = await fetch(url, { redirect: "follow" });
  if (!res.ok) {
    throw new Error(`GET ${url} failed with ${res.status} ${res.statusText}`);
  }
  return Buffer.from(await res.arrayBuffer());
}

// The checksums file is part of the release, so verifying against it catches a
// corrupted download or a truncated CDN response. It is not a defence against
// a compromised release — that would need signature verification, which is on
// the roadmap.
function verifyChecksum(archive, checksums, name) {
  const expected = checksums
    .split("\n")
    .map((line) => line.trim().split(/\s+/))
    .find(([, file]) => file === name)?.[0];

  if (!expected) {
    throw new Error(`${name} is not listed in the release checksums`);
  }

  const actual = createHash("sha256").update(archive).digest("hex");
  if (actual !== expected) {
    throw new Error(
      `checksum mismatch for ${name}\n  expected ${expected}\n  actual   ${actual}`,
    );
  }
}

// tar and unzip are present on every platform Node supports: tar ships with
// Windows 10 1803+ and with every macOS and Linux distribution. Shelling out
// to them avoids adding an extraction dependency to a package whose whole
// purpose is to deliver one file.
function extract(archivePath, windows) {
  const [cmd, args] = windows
    ? ["tar", ["-xf", archivePath, "-C", BIN_DIR]]
    : ["tar", ["-xzf", archivePath, "-C", BIN_DIR]];

  const result = spawnSync(cmd, args, { stdio: "inherit" });
  if (result.error || result.status !== 0) {
    throw new Error(`failed to extract ${archivePath}: ${result.error ?? `exit ${result.status}`}`);
  }
}

async function main() {
  const { os, arch, windows } = target();

  const ext = windows ? "zip" : "tar.gz";
  const name = `doctordock_${VERSION}_${os}_${arch}.${ext}`;
  const base = `https://github.com/${REPO}/releases/download/v${VERSION}`;

  await mkdir(BIN_DIR, { recursive: true });

  console.log(`Downloading DoctorDock ${VERSION} for ${os}/${arch}...`);

  const [archive, checksums] = await Promise.all([
    download(`${base}/${name}`),
    download(`${base}/checksums.txt`).then((b) => b.toString("utf8")),
  ]);

  verifyChecksum(archive, checksums, name);

  const archivePath = join(BIN_DIR, name);
  await writeFile(archivePath, archive);

  try {
    extract(archivePath, windows);
  } finally {
    await rm(archivePath, { force: true });
  }

  const binary = join(BIN_DIR, windows ? "doctordock.exe" : "doctordock");
  if (!windows) {
    await chmod(binary, 0o755);
  }

  console.log(`DoctorDock ${VERSION} installed. Run \`npx doctordock\` or \`npx ddock\`.`);
}

main().catch((err) => {
  console.error(`\nDoctorDock install failed: ${err.message}\n`);
  process.exit(1);
});
