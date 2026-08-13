# ADR-0006: Cleanup is opt-in, staged by risk, and never cascades

- **Status:** accepted
- **Date:** 2026-08-13
- **Supersedes:** the "read-only in v0.1" constraint in [PLAN.md](../PLAN.md)

## Context

DoctorDock reports reclaimable resources: dangling images, unused images,
stopped containers, unused networks, unused volumes. On a real developer
machine that is regularly several gigabytes. Reporting it and then telling the
user to go run `docker image prune` themselves is a worse product than doing it
for them.

The original plan made v0.1 strictly read-only, on the reasoning that a
diagnostics tool that deletes things is a diagnostics tool that will eventually
delete the wrong thing. That reasoning is still correct. It is an argument for
*how* to delete, not for refusing to.

The failure mode that matters is specific and well documented across every tool
in this space: **`docker volume prune` destroying the only copy of a database
somebody needed.** An unused volume and an abandoned volume look identical from
the API. `docker system prune --volumes` has ruined a lot of afternoons.

## Decision

`doctordock cleanup` removes resources, under five constraints.

### 1. Dry run is the default

`doctordock cleanup` alone never deletes anything. It prints what it *would*
remove and how much space that frees. Deleting requires `--apply`.

The verb and the effect are separated on purpose: a user who types the obvious
command and hits enter must not lose data.

### 2. Volumes are never included in any "everything" flag

`--all` covers containers, images and networks. It does **not** cover volumes.
Removing a volume requires typing `--volumes` explicitly, every time.

This is the one asymmetry in the design and it is deliberate. Every other
resource can be recreated: an image can be pulled or rebuilt, a container
recreated, a network re-declared. A volume's contents cannot.

### 3. Every item carries a risk level

| Risk | Meaning | Examples |
|---|---|---|
| `safe` | Docker's own prune would remove it, and it cannot be needed again | dangling images, unused networks |
| `review` | Removable, but you may have wanted it | unused tagged images, stopped containers |
| `data-loss` | May destroy the only copy of real data | unused volumes |

The confirmation prompt states the counts per risk level before asking. A plan
containing `data-loss` items requires typing the word `delete`, not `y`.

### 4. Removal never cascades

Containers are removed with `RemoveVolumes: false`. Docker's `docker rm -v`
would take anonymous volumes with the container, which would route around the
`--volumes` gate entirely — the user would lose a volume they never approved.

Images are removed with `Force: false`. If the daemon refuses because something
started using the image between the scan and the apply, that refusal is the
point; the error is reported rather than forced through.

### 5. The read-only guarantee stays structural

`docker.Client` has no mutating methods and never will. Removal lives on a
separate `docker.Pruner` interface, which only the cleanup command asks for.

This is not documentation, it is the type system: `scanner` is handed a
`Client`, so no scan can delete anything even if someone later writes code that
tries to.

## Consequences

- Cleanup plans the whole operation before touching anything, and accounts for
  ordering: an image referenced only by a stopped container that is *also* being
  removed is correctly treated as unused. Otherwise users would have to run the
  command twice to converge.
- `--keep-since` (default 0) protects recent work. `--keep-since 24h` will not
  remove an image built this morning.
- The cleanup plan is a versioned JSON document like the scan report, so the
  macOS app can present the same "here is what would go" screen before asking.
- CONTRIBUTING's ground rule 5 changes from "read-only" to "no cleanup without
  an explicit `--apply` and no volume removal without an explicit `--volumes`".
  The constraint got more specific, not weaker.
