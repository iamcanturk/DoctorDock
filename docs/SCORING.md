# Health score

The score is a single number in `[0, 100]` summarising how much is wrong with a
Docker environment. It is deliberately simple, and it is deliberately isolated
in `internal/score` behind one interface:

```go
type Scorer interface {
	Calculate(findings []model.Finding) int
}
```

Everything below can be replaced without touching a rule, the scanner or a
renderer.

## What it is not

It is **not a risk model**. It does not know how exposed your host is, whether
a container holds anything valuable, or whether the "unused" volume it found is
your only backup. Two environments with the same score are not equally safe.

What it *is* good for: **comparing a machine to itself over time**. Fixing the
Docker socket mount should move the number, and it does.

## How it works

### 1. Each finding accrues a penalty by severity

| Severity | Weight |
|---|---|
| `CRITICAL` | 25 |
| `HIGH` | 15 |
| `MEDIUM` | 8 |
| `LOW` | 3 |
| `INFO` | 0 |

`INFO` is weightless on purpose. "You have 50 unused images" is worth telling
someone; it is not worth a lower health score.

### 2. Repeats of the same rule decay harmonically

The *n*-th finding from a given rule costs `weight / n`.

On a real developer machine it is normal for fifteen containers to run as root.
Charging full price for each says the same thing fifteen times, and a score
that cannot tell "fifteen root containers" apart from "fifteen root containers
plus an exposed Docker socket" tells the user nothing.

With decay, the first occurrence costs full weight and the tenth costs a tenth.
Breadth still matters — fifteen is worse than one — but one systemic pattern
cannot drown out an unrelated critical finding.

The counter is **per rule**, not global. Ten unused images decay against each
other; they do not discount a privileged container.

### 3. The total is mapped exponentially

```
score = round(100 × e^(−penalty / 100))
```

This is the part that departs from the obvious `100 − penalty`, and it was
driven by running against real environments rather than fixtures.

A working developer laptop easily accumulates 200 penalty points. Plain
subtraction pins it at 0 — the same as a machine that is genuinely ten times
worse, and with no way to show improvement. A score that reads 0 for every real
environment carries no information.

The exponential curve preserves ordering, never saturates, and means every fix
moves the number:

| Penalty | Score | Reads as |
|---|---|---|
| 0 | 100 | excellent |
| 25 | 78 | good |
| 50 | 61 | needs attention |
| 100 | 37 | poor |
| 200 | 14 | critical |
| 400 | 2 | critical |

## Grades

| Score | Grade |
|---|---|
| 90–100 | excellent |
| 75–89 | good |
| 50–74 | needs attention |
| 25–49 | poor |
| 0–24 | critical |

## Comparing scans

To compare two environments or two points in time, compare the **penalty**, not
the score. Penalty is additive; score is not.

```go
scorer := score.Default()
before := scorer.Penalty(oldReport.Findings)
after  := scorer.Penalty(newReport.Findings)
```

## Changing it

Set a different scorer on `scanner.Config`:

```go
scanner.New(client, scanner.Config{
	Scorer: score.Weighted{
		Weights: score.Weights{model.SeverityHigh: 30},
		Scale:   150,
	},
})
```

`Scale` controls how quickly the score falls: at `penalty == Scale` the score is
`100/e ≈ 37`. A larger scale is more forgiving.

## Known limitations

- **Environment size is not normalized.** A host running 200 containers will
  score lower than one running 5, even at the same standard of care. Dividing
  by resource count was rejected for v0.1 because it makes a single critical
  finding on a large host nearly invisible.
- **Findings are independent.** A privileged container that also mounts the
  Docker socket is scored as two problems, though the second adds nothing once
  you have the first.
- **No exposure weighting.** A database published on `0.0.0.0` scores the same
  on a laptop behind NAT as on a public cloud host. Detecting that difference
  would need network probing, which would break the offline guarantee.

These are the things v0.2 should address, in this order.
