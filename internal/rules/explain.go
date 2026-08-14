package rules

// Explanation is the long-form answer to "what is DD005 and why should I care".
//
// The one-line Description a rule carries is enough for a table. It is not
// enough for somebody who has just been told their container is insecure and
// wants to know what that actually means before changing anything in
// production.
// The json tags matter: `doctordock explain --format json` is consumed by the
// macOS app, so this shares the snake_case convention of the rest of the
// contract rather than leaking Go field names.
type Explanation struct {
	// What the rule looks for, in plain terms.
	What string `json:"what"`
	// Why it matters — the consequence, not the principle.
	Why string `json:"why"`
	// Scenario is a concrete walk-through of what goes wrong. Empty when the
	// rule is not about an attack.
	Scenario string `json:"scenario,omitempty"`
	// Fixes are ordered best-first.
	Fixes []Fix `json:"fixes"`
	// FalsePositives explains when the finding is fine to ignore. Being honest
	// about this is what stops people ignoring the whole tool.
	FalsePositives string      `json:"false_positives,omitempty"`
	References     []Reference `json:"references,omitempty"`
}

// Fix is one concrete way to resolve a finding.
type Fix struct {
	Title string `json:"title"`
	// Lang is the syntax highlight hint: "bash", "dockerfile" or "yaml".
	Lang string `json:"lang"`
	Code string `json:"code"`
}

// Reference points at authoritative documentation. DoctorDock never fetches
// these — it only prints them.
type Reference struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Explain returns the long-form explanation for a rule ID.
//
// Explanations live in one table rather than on the Rule interface so that
// adding a rule stays a two-line change. A test asserts every registered rule
// has an entry, which is what stops the table drifting from the registry.
func Explain(id string) (Explanation, bool) {
	e, ok := explanations[id]
	return e, ok
}

var explanations = map[string]Explanation{
	"DD001": {
		What: "The container's processes start as uid 0 — root inside the container. " +
			"That happens when neither the container nor its image sets a USER.",
		Why: "Container root is not host root, but it is the starting point for almost every " +
			"escape technique. A compromised process that is already root can write anywhere in " +
			"the container filesystem, install tools, and make full use of whatever capabilities " +
			"and mounts the container has. Running as an unprivileged user removes that first step.",
		Scenario: "An attacker finds a file-upload bug in your web app. As root they overwrite " +
			"the application binary and wait for a restart. As uid 1000 they cannot write to it at all.",
		Fixes: []Fix{
			{
				Title: "Set a non-root user in the image (best — applies everywhere the image runs)",
				Lang:  "dockerfile",
				Code: "RUN addgroup -S app && adduser -S -G app app\n" +
					"# Give the user ownership of anything it must write to\n" +
					"RUN chown -R app:app /app\n" +
					"USER app",
			},
			{
				Title: "Override at runtime when you cannot change the image",
				Lang:  "bash",
				Code:  "docker run --user 1000:1000 myimage",
			},
			{
				Title: "In Compose",
				Lang:  "yaml",
				Code:  "services:\n  api:\n    user: \"1000:1000\"",
			},
		},
		FalsePositives: "Many official images (postgres, mysql, nginx) start as root and drop " +
			"privileges in their entrypoint. DoctorDock reads configuration, not runtime state, so " +
			"it cannot see that. If you have verified the process actually runs unprivileged, " +
			"suppress it: `--ignore DD001`.",
		References: []Reference{
			{"Docker: run as a non-root user", "https://docs.docker.com/build/building/best-practices/#user"},
			{"CIS Docker Benchmark 4.1", "https://www.cisecurity.org/benchmark/docker"},
		},
	},

	"DD002": {
		What: "The container was started with --privileged.",
		Why: "Privileged mode grants every Linux capability, disables the seccomp and AppArmor " +
			"profiles, and gives the container access to every host device. It is not a stronger " +
			"container — it is effectively no container. Escaping to the host from one is a " +
			"well-documented, few-line exercise.",
		Scenario: "A privileged container can mount the host's root disk (`mount /dev/sda1 /mnt`), " +
			"write to /mnt/etc/crontab, and execute arbitrary commands as host root a minute later. " +
			"No kernel exploit involved.",
		Fixes: []Fix{
			{
				Title: "Remove it and grant only what is actually needed",
				Lang:  "bash",
				Code: "# instead of --privileged\n" +
					"docker run --cap-drop ALL --cap-add NET_BIND_SERVICE myimage\n\n" +
					"# need one device rather than all of them?\n" +
					"docker run --device /dev/ttyUSB0 myimage",
			},
			{
				Title: "Find out which capability the workload really wants",
				Lang:  "bash",
				Code: "# run it without privileged and read the failure — the error names\n" +
					"# the operation, which maps to a capability\n" +
					"docker run --cap-drop ALL myimage",
			},
		},
		FalsePositives: "Docker-in-Docker and some hardware-access workloads genuinely need it. " +
			"If so, treat that container as part of the host's trust boundary, not as isolated.",
		References: []Reference{
			{"Docker: runtime privilege", "https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities"},
		},
	},

	"DD003": {
		What: "The container shares the host's network namespace (--network host).",
		Why: "There is no network isolation and no port mapping. Every port the container opens " +
			"is opened directly on the host, it can reach services bound to the host's loopback " +
			"interface — which are usually the ones assumed to be unreachable — and it can observe " +
			"host network traffic.",
		Scenario: "A database bound to 127.0.0.1 is considered safe from containers. A container " +
			"on host networking connects to it directly, because 127.0.0.1 is the same interface.",
		Fixes: []Fix{
			{
				Title: "Use a user-defined network and publish only what you need",
				Lang:  "bash",
				Code: "docker network create app-net\n" +
					"docker run --network app-net -p 8080:80 myimage",
			},
		},
		FalsePositives: "High-throughput proxies, network monitoring tools and some VPN sidecars " +
			"legitimately need it. On Docker Desktop host networking behaves differently from Linux " +
			"and is often a workaround rather than a requirement.",
		References: []Reference{
			{"Docker: host networking", "https://docs.docker.com/engine/network/drivers/host/"},
		},
	},

	"DD004": {
		What: "A sensitive host path is bind-mounted into the container — /etc, /var/lib/docker, " +
			"~/.ssh, ~/.aws or similar.",
		Why: "A bind mount is not a copy. The container reads and, unless mounted read-only, " +
			"writes the real host files. Credential directories are the common case and the " +
			"expensive one: a container with ~/.aws mounted has your AWS account.",
		Scenario: "A build container mounts the whole home directory for convenience. A compromised " +
			"dependency in that build reads ~/.ssh/id_rsa and ~/.aws/credentials, and now has your " +
			"production access rather than just your build output.",
		Fixes: []Fix{
			{
				Title: "Narrow the mount to the specific path the container needs",
				Lang:  "bash",
				Code: "# instead of the whole home directory\n" +
					"docker run -v ~/project/src:/src:ro myimage",
			},
			{
				Title: "Make it read-only when the container only needs to read",
				Lang:  "bash",
				Code:  "docker run -v /etc/timezone:/etc/timezone:ro myimage",
			},
		},
		FalsePositives: "Mounting /etc/localtime or /etc/timezone read-only is a normal way to fix " +
			"container clocks. Monitoring agents legitimately mount /proc and /sys read-only.",
		References: []Reference{
			{"Docker: bind mounts", "https://docs.docker.com/engine/storage/bind-mounts/"},
		},
	},

	"DD005": {
		What: "The Docker socket (/var/run/docker.sock) is mounted into the container.",
		Why: "Access to the Docker socket is root on the host. Anyone who can talk to it can start " +
			"a new privileged container that mounts the host filesystem. Mounting it read-only does " +
			"not help — it is an API endpoint, not a file, and every dangerous operation is a POST.",
		Scenario: "A container with the socket runs one API call to launch a second container with " +
			"`--privileged -v /:/host`, then writes an SSH key into /host/root/.ssh/authorized_keys. " +
			"Total elapsed time: seconds. This is the single highest-impact finding DoctorDock reports.",
		Fixes: []Fix{
			{
				Title: "Remove the mount",
				Lang:  "bash",
				Code:  "# drop: -v /var/run/docker.sock:/var/run/docker.sock",
			},
			{
				Title: "If the container genuinely needs the API, put a filtering proxy in front",
				Lang:  "yaml",
				Code: "services:\n" +
					"  docker-proxy:\n" +
					"    image: tecnativa/docker-socket-proxy\n" +
					"    environment:\n" +
					"      CONTAINERS: 1     # allow only what is needed\n" +
					"      POST: 0           # refuse every mutating call\n" +
					"    volumes:\n" +
					"      - /var/run/docker.sock:/var/run/docker.sock:ro\n\n" +
					"  watchtower:\n" +
					"    environment:\n" +
					"      DOCKER_HOST: tcp://docker-proxy:2375",
			},
		},
		FalsePositives: "None worth relying on. Watchtower, Portainer and Traefik all ask for the " +
			"socket, and all of them are equivalent to giving that container host root. If you accept " +
			"that, treat the container as part of the host.",
		References: []Reference{
			{"Docker socket security", "https://docs.docker.com/engine/security/protect-access/"},
			{"docker-socket-proxy", "https://github.com/Tecnativa/docker-socket-proxy"},
		},
	},

	"DD006": {
		What: "A database, message broker or admin port is published on 0.0.0.0 — every network " +
			"interface — rather than bound to loopback.",
		Why: "`-p 5432:5432` does not mean \"reachable from my machine\". It means reachable from " +
			"anything that can route to your machine: other devices on the café Wi-Fi, other tenants " +
			"on the same cloud subnet, and on a host without a firewall, the internet. Docker also " +
			"writes its own iptables rules, so a published port frequently bypasses ufw.",
		Scenario: "A Redis container published on 0.0.0.0:6379 with no password is found by an " +
			"internet-wide scan within hours. Redis can write files; the standard follow-up writes an " +
			"SSH key or a cron job.",
		Fixes: []Fix{
			{
				Title: "Bind to loopback if only your machine needs it",
				Lang:  "bash",
				Code:  "docker run -p 127.0.0.1:5432:5432 postgres:16",
			},
			{
				Title: "Do not publish at all if only other containers need it",
				Lang:  "yaml",
				Code: "services:\n" +
					"  db:\n" +
					"    image: postgres:16\n" +
					"    # no ports: — other services reach it as db:5432 on the shared network\n" +
					"  api:\n" +
					"    environment:\n" +
					"      DATABASE_URL: postgres://db:5432/app",
			},
		},
		FalsePositives: "A port that is meant to be public — a web server on 80 or 443 — is not " +
			"reported. Only datastore and admin ports are.",
		References: []Reference{
			{"Docker and iptables", "https://docs.docker.com/engine/network/packet-filtering-firewalls/"},
		},
	},

	"DD007": {
		What: "The container declares no HEALTHCHECK, so Docker only knows whether its main " +
			"process is alive.",
		Why: "\"Alive\" and \"working\" are different. A process that has deadlocked, lost its " +
			"database connection or filled its thread pool stays Up indefinitely. Nothing restarts " +
			"it, orchestrators keep routing to it, and dependent services get errors instead of a " +
			"clear signal to wait.",
		Fixes: []Fix{
			{
				Title: "Add one to the image",
				Lang:  "dockerfile",
				Code: "HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \\\n" +
					"  CMD curl -fsS http://localhost:8080/healthz || exit 1",
			},
			{
				Title: "Or in Compose, with dependants waiting for it",
				Lang:  "yaml",
				Code: "services:\n" +
					"  db:\n" +
					"    image: postgres:16\n" +
					"    healthcheck:\n" +
					"      test: [\"CMD-SHELL\", \"pg_isready -U postgres\"]\n" +
					"      interval: 10s\n" +
					"      retries: 5\n" +
					"  api:\n" +
					"    depends_on:\n" +
					"      db:\n" +
					"        condition: service_healthy",
			},
		},
		FalsePositives: "One-shot jobs and init containers do not need one. Neither do containers " +
			"whose health an external supervisor already tracks.",
		References: []Reference{
			{"Dockerfile HEALTHCHECK", "https://docs.docker.com/reference/dockerfile/#healthcheck"},
		},
	},

	"DD008": {
		What: "The container has no restart policy, so it stays down after a crash, a daemon " +
			"restart or a host reboot.",
		Why: "Silent downtime. The container will not come back, and nothing reports that it did " +
			"not — you find out when something else fails.",
		Fixes: []Fix{
			{
				Title: "unless-stopped for long-running services",
				Lang:  "bash",
				Code: "docker run --restart unless-stopped myimage\n\n" +
					"# unless-stopped vs always: `always` restarts even a container you\n" +
					"# stopped on purpose, after the daemon restarts. `unless-stopped`\n" +
					"# respects your decision.",
			},
			{
				Title: "In Compose",
				Lang:  "yaml",
				Code:  "services:\n  api:\n    restart: unless-stopped",
			},
		},
		FalsePositives: "Correct for one-shot jobs, migrations and anything run with --rm. This is " +
			"reported at INFO precisely because it is often intentional.",
		References: []Reference{
			{"Docker restart policies", "https://docs.docker.com/engine/containers/start-containers-automatically/"},
		},
	},

	"DD009": {
		What: "The container was granted Linux capabilities that weaken or defeat isolation — " +
			"SYS_ADMIN, SYS_MODULE, SYS_PTRACE, NET_ADMIN and similar.",
		Why: "Capabilities split root's powers into pieces, which is useful when you add one and " +
			"dangerous when you add the wrong one. CAP_SYS_ADMIN is close to full root: it allows " +
			"mounting filesystems and manipulating namespaces, the primitives most container escapes " +
			"are built on. CAP_SYS_MODULE loads kernel modules, which is unrestricted host code " +
			"execution.",
		Scenario: "A container with SYS_ADMIN can mount a cgroup filesystem and use the release_agent " +
			"mechanism to run a script as host root. It is a handful of shell lines and needs no exploit.",
		Fixes: []Fix{
			{
				Title: "Drop everything, then add back only what fails",
				Lang:  "bash",
				Code: "docker run --cap-drop ALL --cap-add NET_BIND_SERVICE myimage\n\n" +
					"# Common legitimate additions:\n" +
					"#   NET_BIND_SERVICE  bind to a port below 1024\n" +
					"#   CHOWN, SETUID, SETGID  entrypoints that drop privileges",
			},
			{
				Title: "In Compose",
				Lang:  "yaml",
				Code:  "services:\n  api:\n    cap_drop: [ALL]\n    cap_add: [NET_BIND_SERVICE]",
			},
		},
		FalsePositives: "NET_ADMIN is genuine for VPN and network tooling. SYS_PTRACE is genuine for " +
			"debuggers and profilers — but should not be on a production service.",
		References: []Reference{
			{"Linux capabilities(7)", "https://man7.org/linux/man-pages/man7/capabilities.7.html"},
			{"Docker: runtime privilege", "https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities"},
		},
	},

	"DD010": {
		What: "A running container has no memory limit, so it can allocate all host memory.",
		Why: "The damage is not confined to the container. When the host runs out of memory the " +
			"kernel OOM killer chooses a victim by its own heuristic, which is regularly a different, " +
			"healthy process — your database rather than the leaking worker.",
		Fixes: []Fix{
			{
				Title: "Set a limit above observed peak usage",
				Lang:  "bash",
				Code: "docker run --memory 512m --memory-swap 512m myimage\n\n" +
					"# Find the peak first:\n" +
					"docker stats --no-stream",
			},
			{
				Title: "In Compose",
				Lang:  "yaml",
				Code:  "services:\n  api:\n    mem_limit: 512m",
			},
		},
		FalsePositives: "On a single-purpose host with one workload, a limit adds little. Setting one " +
			"too low turns a slow leak into a hard crash, so measure before choosing a number.",
		References: []Reference{
			{"Docker resource constraints", "https://docs.docker.com/engine/containers/resource_constraints/"},
		},
	},

	"DD011": {
		What: "The container runs from a moving tag — :latest, :main, :stable — rather than a " +
			"pinned version or digest.",
		Why: "A moving tag means the same reference produces different code over time. Two hosts " +
			"pulling \"the same\" image can run different builds, a rollback has nothing to roll back " +
			"to, and a compromised upstream tag reaches you on the next pull.",
		Fixes: []Fix{
			{
				Title: "Pin to a version",
				Lang:  "bash",
				Code:  "docker run nginx:1.27.3    # not nginx:latest",
			},
			{
				Title: "Pin to a digest when you need it to be exactly reproducible",
				Lang:  "bash",
				Code: "docker run nginx@sha256:0c86dddac19f2ce4fd716ac58c0fd87bf...\n\n" +
					"# Find the digest of what you are running now:\n" +
					"docker inspect --format '{{index .RepoDigests 0}}' nginx:1.27.3",
			},
		},
		FalsePositives: "Fine for local experimentation. It is reported at INFO for that reason — " +
			"the cost only appears when you need to reproduce or roll back.",
		References: []Reference{
			{"Docker: pin base image versions", "https://docs.docker.com/build/building/best-practices/#pin-base-image-versions"},
		},
	},

	"DD012": {
		What: "The container is running and its own healthcheck is currently failing.",
		Why: "The container declared what \"working\" means and it is not meeting it. Docker keeps " +
			"it running and dependants keep calling it, so this is usually a live incident rather " +
			"than a configuration warning.",
		Fixes: []Fix{
			{
				Title: "Read what the check actually reported",
				Lang:  "bash",
				Code: "docker inspect --format '{{json .State.Health}}' CONTAINER | jq\n" +
					"docker logs --tail 100 CONTAINER",
			},
			{
				Title: "Rule out the check itself being wrong",
				Lang:  "bash",
				Code: "# a healthcheck using curl in an image without curl fails forever\n" +
					"docker exec CONTAINER sh -c 'command -v curl wget'",
			},
		},
		References: []Reference{
			{"Dockerfile HEALTHCHECK", "https://docs.docker.com/reference/dockerfile/#healthcheck"},
		},
	},

	"DD013": {
		What: "The container is stuck in the restarting state, or has been restarted many times.",
		Why: "The process exits shortly after starting and the restart policy keeps bringing it " +
			"back. Each cycle drops in-flight work, and a container that never stays up long enough " +
			"to become healthy produces a steady stream of errors for anything depending on it.",
		Fixes: []Fix{
			{
				Title: "Read the exit reason — this is almost always in the logs",
				Lang:  "bash",
				Code: "docker logs --tail 100 CONTAINER\n" +
					"docker inspect --format '{{.State.ExitCode}} {{.State.Error}}' CONTAINER\n\n" +
					"# Exit 137 means the kernel killed it — usually out of memory (see DD010)\n" +
					"# Exit 1 with no logs often means a missing environment variable",
			},
		},
		References: []Reference{
			{"Docker restart policies", "https://docs.docker.com/engine/containers/start-containers-automatically/"},
		},
	},

	"DD014": {
		What: "An image with no tag and no digest — the leftover of a build whose tag was moved to " +
			"a newer image.",
		Why: "Nothing can reference it again, so it is pure disk usage. On a machine that builds " +
			"regularly these accumulate quickly.",
		Fixes: []Fix{
			{
				Title: "Remove them",
				Lang:  "bash",
				Code: "doctordock cleanup --apply    # dangling images and unused networks\n" +
					"docker image prune            # the same thing, Docker's own command",
			},
		},
		FalsePositives: "None. This is exactly what `docker image prune` removes, and DoctorDock " +
			"uses the same definition so the two never disagree.",
		References: []Reference{
			{"docker image prune", "https://docs.docker.com/reference/cli/docker/image/prune/"},
		},
	},

	"DD015": {
		What: "A tagged image that no container references, running or stopped.",
		Why: "It occupies disk without being used. Whether that matters depends on whether it is a " +
			"base image you rebuild against every day or something from a project you finished " +
			"months ago — which is why this is INFO rather than a warning.",
		Fixes: []Fix{
			{
				Title: "Review what is actually large before removing anything",
				Lang:  "bash",
				Code: "doctordock images                       # sizes and what references what\n" +
					"doctordock cleanup --images             # what would go, removes nothing\n" +
					"doctordock cleanup --images --apply     # remove them",
			},
			{
				Title: "Keep recent work",
				Lang:  "bash",
				Code:  "doctordock cleanup --images --keep-since 168h --apply   # keep the last week",
			},
		},
		FalsePositives: "Base images you pull constantly show up here. Removing them costs a " +
			"download, not data.",
	},

	"DD016": {
		What: "An image above the size threshold — 1.5 GB by default, configurable.",
		Why: "Size is paid on every pull, every cold start, every registry push and every CI run " +
			"that does not hit the cache. A larger base image also means more installed packages, " +
			"which means a larger surface to keep patched.",
		Fixes: []Fix{
			{
				Title: "Find out which layer is responsible first",
				Lang:  "bash",
				Code:  "docker history --no-trunc --human IMAGE | head -20",
			},
			{
				Title: "Multi-stage build: compile in one stage, ship only the result",
				Lang:  "dockerfile",
				Code: "FROM golang:1.25 AS build\n" +
					"WORKDIR /src\n" +
					"COPY . .\n" +
					"RUN CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o /app ./cmd/api\n\n" +
					"FROM gcr.io/distroless/static-debian12\n" +
					"COPY --from=build /app /app\n" +
					"USER nonroot:nonroot\n" +
					"ENTRYPOINT [\"/app\"]",
			},
			{
				Title: "Stop copying what you never need",
				Lang:  "bash",
				Code: "# .dockerignore\n" +
					".git\nnode_modules\n**/*.log\ndist\ncoverage",
			},
		},
		FalsePositives: "CUDA, JDK and full data-science images are legitimately large. Raise the " +
			"threshold rather than ignoring the rule:\n\n" +
			"    thresholds:\n      large_image_bytes: 4000000000",
		References: []Reference{
			{"Dockerfile best practices", "https://docs.docker.com/build/building/best-practices/"},
		},
	},

	"DD017": {
		What: "A volume that no container mounts. Anonymous ones — created implicitly by a " +
			"container and never named — are called out separately.",
		Why: "Volumes are where disk quietly disappears to. An anonymous volume outlives the " +
			"container that created it and nothing names it, so it is never obviously anybody's.",
		Fixes: []Fix{
			{
				Title: "Look inside before deciding — this is the one that cannot be undone",
				Lang:  "bash",
				Code: "doctordock volumes                     # names, drivers, what mounts what\n" +
					"docker run --rm -v VOLUME:/v alpine ls -la /v\n" +
					"docker run --rm -v VOLUME:/v alpine du -sh /v",
			},
			{
				Title: "Remove only when you are sure",
				Lang:  "bash",
				Code: "docker volume rm VOLUME\n\n" +
					"# DoctorDock requires --volumes explicitly; --all never includes them\n" +
					"doctordock cleanup --volumes            # review first, removes nothing\n" +
					"doctordock cleanup --volumes --apply    # asks you to type \"delete\"",
			},
		},
		FalsePositives: "A stopped project's database volume looks exactly like an abandoned one. " +
			"This is why DoctorDock will never remove a volume without being asked twice.",
		References: []Reference{
			{"Docker volumes", "https://docs.docker.com/engine/storage/volumes/"},
		},
	},

	"DD018": {
		What: "A user-defined network with no containers attached. Docker's predefined networks " +
			"(bridge, host, none, ingress) are never reported.",
		Why: "Compose leaves these behind when a project is brought down. Each one holds an address " +
			"range from the pool Docker allocates bridge networks from, and that pool is finite — " +
			"eventually `docker network create` starts failing with \"no available IPv4 addresses\".",
		Fixes: []Fix{
			{
				Title: "Remove them",
				Lang:  "bash",
				Code: "doctordock cleanup --apply    # included in the safe defaults\n" +
					"docker network prune",
			},
		},
		FalsePositives: "A network you created ahead of the containers that will join it is " +
			"reported. Harmless — this is INFO.",
		References: []Reference{
			{"Docker networking", "https://docs.docker.com/engine/network/"},
		},
	},
}
