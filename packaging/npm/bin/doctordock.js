#!/usr/bin/env node
// Thin launcher: forwards every argument and the exit code to the real binary
// that install.js downloaded.
//
// Forwarding the exit code faithfully is the whole point — `doctordock scan
// --fail-on high` in a CI pipeline is worthless if the wrapper swallows the 2.

import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const binary = join(here, process.platform === "win32" ? "doctordock.exe" : "doctordock");

if (!existsSync(binary)) {
  console.error(
    "DoctorDock binary is missing. Reinstall the package, or run the postinstall step:\n" +
      "  node node_modules/doctordock/install.js",
  );
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error(`Failed to run DoctorDock: ${result.error.message}`);
  process.exit(1);
}

// A binary killed by a signal has a null status; report it the way a shell
// would so that Ctrl-C is not reported as success.
if (result.status === null) {
  process.exit(result.signal ? 128 : 1);
}

process.exit(result.status);
