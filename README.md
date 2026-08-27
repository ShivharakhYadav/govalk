# govalk (working name)

> Stub written pre-event for the Zero Dependency Hackathon (Aug 28–31,
> 2026). Real content — usage, examples, honest limits — gets filled in
> during the build. See PLAN.md and REQUIREMENTS.md for the full design
> and submission checklist.

A Redis-protocol-compatible, in-memory key-value store with crash-safe
append-only-file persistence. Zero third-party runtime dependencies —
Go standard library only.

## Status

Pre-hackathon planning only. No implementation code has been written
(per event Rule #4: new code only, written during the 72-hour window).

## Track

D — Data & Storage

## Planned usage (subject to change, see PLAN.md)

```
make build
./bin/govalk --port 6379 --aof ./data.aof
redis-cli -p 6379 SET foo bar
redis-cli -p 6379 GET foo
```

## Docs

- [PLAN.md](./PLAN.md) — architecture, scope, timeline
- [REQUIREMENTS.md](./REQUIREMENTS.md) — submission checklist against
  official hackathon rules
- [STDLIB.md](./STDLIB.md) — stdlib-for-package substitution log
