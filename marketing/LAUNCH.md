# DoctorDock — Social Launch Kit

Everything below is copy-paste ready. Every claim is verified against the repo. Attach the images where marked (the share card lives at `web/assets/health-card.png`; app screenshots at `web/assets/app-overview.png`, `app-finding.png`, `app-popover.png`).

---

## 1. Launch sequence ("kurgu")

Keep it to one channel per beat so you can actually reply to every comment. Front-load the launch on a Tuesday–Thursday.

- **Day 0 — morning (US Eastern, ~8:30–10:00 a.m. ET).** Post the **X thread** first. Immediately after it's live, submit **Show HN**. The two feed each other — put the HN link in your own X replies once it's up, and pin the thread. Spend the day at the keyboard answering.
- **Day 1 — Reddit.** Post to **r/docker** in the morning, then **r/selfhosted** a few hours later (don't cross-post the identical body — each has its own angle below). Lead with the problem, link second.
- **Day 2 — LinkedIn.** The reflective "why I built it" version for a professional audience. Reuse a screenshot + the score card.
- **Day 2–3 — Turkish audience.** Drop the Turkish hook tweet + standalone when your TR followers are online (evening in Türkiye).
- **When genuinely ready — Product Hunt.** Only after the repo README, landing page, and a clean `brew install` are all solid. PH goes live 12:01 a.m. PT; line up a few honest early users. Don't rush this to Day 0.

Rule of thumb: one launch surface per day, reply to everything, never paste the same block twice.

---

## 2. X (Twitter) launch thread

> **Tweet 1 — hook** · *attach the score-card image*

```
Your Docker setup has problems you can't see.

A container mounted to host root. A database open to the whole network. A dozen containers running as root. 12 GB of images nothing uses.

DoctorDock finds all of it in ~550ms. Fully offline.

A doctor for your Docker.
```

> **Tweet 2 — No AI**

```
No AI.

Every finding is deterministic Go you can read. Same environment in, same report out. No model, no vibes, no "this might be insecure."

An AI-free dev tool, on purpose.
```

> **Tweet 3 — Fully offline**

```
Fully offline.

Zero network calls. No telemetry. No account. No CVE feed to sync.

It runs air-gapped and inside a locked-down CI runner. Nothing about your environment ever leaves the machine it's on.
```

> **Tweet 4 — Privacy-first**

```
Privacy-first.

It reads container env vars as key NAMES only — the values never enter a report. Safe to point at production.

The shareable score card is aggregate numbers only: no container name, image tag, port, or path.
```

> **Tweet 5 — Guarded cleanup**

```
Guarded cleanup.

`doctordock cleanup` is a dry run by default. Removing anything needs `--apply`. A Docker volume is never touched unless you ask for it explicitly — and then you type `delete`, not `y`.

Everything else can be recreated. A volume's data can't.
```

> **Tweet 6 — Fast + score** · *attach an app screenshot (menubar + findings panel)*

```
Fast, and it ends with a number you can act on.

A full scan — containers, images, volumes, networks — takes ~550ms and produces a Docker health score out of 100.

There's a native macOS menubar app too, showing the score, colour-coded, at a glance.
```

> **Tweet 7 — How it works / install**

```
How it works: it reads your local Docker socket, runs 18 config rules, and prints what's wrong plus how to fix each one.

It's not a CVE scanner — that's Trivy/Grype. This is the config layer they skip.

macOS:
brew install iamcanturk/tap/doctordock
```

> **Tweet 8 — CTA**

```
Free and open source, MIT. No account, nothing to sign up for.

Code: https://github.com/iamcanturk/DoctorDock
Site: https://doctordock.iamcanturk.dev

If you run Docker locally, point it at your machine and tell me what it finds.
```

---

## 3. Single standalone tweet (reusable)

```
DoctorDock — a doctor for your Docker.

Scans your local Docker for security problems, misconfigs, and reclaimable disk in ~550ms. Fully offline, no AI, no telemetry, no account. Open source (MIT).

brew install iamcanturk/tap/doctordock

https://github.com/iamcanturk/DoctorDock
```

---

## 4. Turkish variant

> **Hook tweet**

```
Docker ortamınızda göremediğiniz sorunlar var: host'a bağlı bir socket, tüm ağa açık bir veritabanı, root ile çalışan bir sürü container, kimsenin kullanmadığı 12 GB imaj.

DoctorDock hepsini saniyenin altında bulur. Tamamen çevrimdışı.

Docker'ınız için bir doktor.
```

> **Standalone tweet**

```
DoctorDock — Docker'ınız için bir doktor.

Yerel Docker ortamınızı güvenlik sorunları, hatalı yapılandırma ve boşa giden disk için ~550ms'de tarar. Tamamen çevrimdışı, yapay zeka yok, telemetri yok. Açık kaynak (MIT).

brew install iamcanturk/tap/doctordock
```

---

## 5. Show HN

**Title:**

```
Show HN: DoctorDock – find Docker misconfigs and wasted disk, offline, no AI
```

**First comment:**

```
Author here. I built this because Docker makes it easy to run things and
hard to notice what you're already running. Six months into a project my
machine had a container mounting the Docker socket (that's host root), a
couple of databases published on 0.0.0.0, a dozen containers running as
root, and gigabytes of images nothing referenced. None of it was visible
until I went looking.

There are good tools for parts of this. Trivy and Grype scan images for
CVEs — DoctorDock deliberately does not, that's a solved problem and it
needs a network. This looks at the configuration layer instead: how your
containers are actually wired up, and what disk you can reclaim. It ends
with a health score out of 100.

Two constraints I cared about:

- No AI. Every finding is deterministic Go you can read; same input, same
  output. Nothing is guessed.
- Fully offline. Zero network calls, no telemetry, no account. It runs
  air-gapped and in CI. Env var values are never read — key names only —
  so it's safe to point at production.

Cleanup is a dry run unless you pass --apply, and a volume is never
removed unless you ask for it by name. A full scan takes ~550ms.

It's MIT, there's a CLI and a native macOS menubar app. macOS install is
`brew install iamcanturk/tap/doctordock`.

Repo: https://github.com/iamcanturk/DoctorDock
I'd genuinely like to hear what it finds on your machine, and where the
rules are wrong or noisy — false positives are the thing I most want to
fix.
```

---

## 6. Reddit

### r/docker

**Title:**

```
I got tired of not knowing what was wrong with my local Docker, so I wrote a scanner for it
```

**Body:**

```
After a while every dev machine accumulates the same mess: a container
with the Docker socket mounted, a database published on 0.0.0.0 instead
of 127.0.0.1, a handful of containers running as root, and images nothing
references eating disk. `docker system df` tells you the disk part with no
opinion; nothing tells you the rest in one shot.

So I built DoctorDock. It reads your local Docker socket, runs 18
configuration rules, and prints what's wrong with a concrete fix for each,
plus a health score out of 100. It also shows what's safely reclaimable —
cleanup is a dry run until you pass --apply, and volumes are never touched
unless you explicitly ask.

Deliberately NOT a CVE scanner — use Trivy/Grype for images. This is the
config layer they don't look at, which is also why it can stay fully
offline: no network calls, no telemetry, no account. Deterministic Go, no
AI. Env var values are never read (key names only).

macOS: brew install iamcanturk/tap/doctordock
One static binary otherwise. MIT.

Repo: https://github.com/iamcanturk/DoctorDock

Curious what it flags for people here — and where a rule is too noisy.
```

### r/selfhosted

**Title:**

```
A local, offline scanner that tells you what's misconfigured (and what's wasting disk) in your Docker setup
```

**Body:**

```
If you self-host a stack of containers, you've probably got things running
that you'd rather not: a service with the Docker socket mounted, a Postgres
or Redis bound to 0.0.0.0 and reachable from your LAN, containers running
as root, plus old images and unused volumes quietly filling the disk.

DoctorDock scans your local Docker and reports exactly that — security
misconfigs, risky exposure, and reclaimable space — with a fix for each
finding and a health score out of 100. A full scan is about half a second.

For a self-hosting crowd the design might matter to you: it's fully
offline (zero network calls, no telemetry, no account, runs air-gapped),
it's deterministic with no AI in the analysis path, and it never reads env
var values — only the key names — so it's safe to run against a live box.
Cleanup won't delete anything without --apply, and a volume is never
removed unless you ask for it by name.

MIT, self-contained binary, native macOS menubar app too.
macOS: brew install iamcanturk/tap/doctordock
https://github.com/iamcanturk/DoctorDock

Would love feedback from people running bigger home stacks than mine.
```

---

## 7. LinkedIn post

```
Docker makes it easy to run things and hard to notice what you're already
running.

Six months into any project, the typical machine has picked up a few
quiet risks: a container with the Docker socket mounted (effectively host
root), a database published to the whole network, a dozen containers
running as root, and gigabytes of images nothing references. None of it is
visible until something breaks or somebody scans you.

So I built DoctorDock — a local-first tool that scans your Docker
environment and reports the security problems, misconfigurations, and
reclaimable disk, then gives you a health score out of 100. A full scan
takes about half a second.

Two decisions I stand behind: no AI in the analysis path — every finding
is deterministic code you can read — and fully offline, with no telemetry
and no account. It's not a CVE scanner; it covers the configuration layer
that Trivy and Grype don't.

It's free and open source under MIT. If you work with Docker, I'd value
your feedback.

Code: https://github.com/iamcanturk/DoctorDock
```

---

## 8. Product Hunt

**Tagline (≤60 chars):**

```
A doctor for your Docker — offline, no AI, open source
```

**Description:**

```
DoctorDock scans your local Docker environment for security problems,
misconfigurations, and reclaimable disk in about half a second — then
gives you a health score out of 100 you can share. It's fully offline
with no AI, no telemetry, and no account: deterministic Go you can read,
safe to point at production. Not a CVE scanner (that's Trivy/Grype) — it
covers the configuration layer they skip. Free and open source, MIT, with
a native macOS menubar app.
```

---

## 9. Hashtags & handles

**Reusable set (pick 3–5 per post, never all):**

```
#Docker #DevOps #DevSecOps #OpenSource #ContainerSecurity #SelfHosted #CLI #Golang #InfoSec
```

**Handles:** post from **@iamcanturk**. Tag relevant accounts *sparingly and only where genuinely relevant* — e.g. @Docker on the container-focused posts. Don't @-stuff Trivy/Grype/other tools into launch copy; mention them by name in the body as the honest comparison instead. One tag per post is plenty; zero is fine.

---

## 10. Reusable one-liners / taglines

```
A doctor for your Docker.

Find the security problems, misconfigs, and wasted disk in your Docker — in under a second, entirely offline.

An AI-free dev tool, on purpose. Deterministic Go you can read.

Zero network calls. No telemetry. No account. It just reads your Docker and tells you the truth.

Not a CVE scanner. The configuration layer Trivy and Grype don't look at.

Reads your env var key names, never the values. Safe to point at production.

Dry run by default. A volume is never deleted unless you ask for it by name.

A Docker health score out of 100 — shareable, and it leaks nothing about your setup.
```
