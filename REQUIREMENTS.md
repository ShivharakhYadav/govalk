# Requirements Checklist — Zero Dependency Hackathon

Source: https://zerodepshack.com/ (fetched 2026-08-25). Re-verify against
the live site closer to kickoff (Aug 26 per their timeline, they post
stdlib cheat-sheets + per-track guidance).

## Event facts
- [ ] Confirm kickoff time in local timezone: Aug 28, 2026, 18:00 UTC
- [ ] Confirm freeze time in local timezone: Aug 31, 2026, 18:00 UTC
- [ ] Team registered (solo or up to 4) — user says already registered
- [ ] Track declared: **D — Data & Storage**

## The one hard rule
- [ ] `go.mod` has an empty `require` block at submission time
- [ ] No import of any module outside Go's standard library, anywhere
      in the shipped artifact
- [ ] No shelling out to a separately-installed tool (e.g. no calling
      a system `redis-server`/`sqlite3` binary — that's a hidden dep)
- [ ] Any dev-only dependency (we don't expect to need one — Go ships
      `testing` in stdlib) disclosed in STDLIB.md if it ever creeps in

## Rule-by-rule (the 9 core rules)
1. [ ] Zero third-party runtime deps — empty manifest, verified
2. [ ] Stdlib definition respected — compiler/build tool/stdlib test
       tool only; no runtime package
3. [ ] Standalone & runnable — single-command build via Makefile,
       tested on a clean checkout (not just dev machine)
4. [ ] **New code only** — nothing committed before Aug 28 18:00 UTC.
       Pre-event allowed: this plan, RESP/AOF format design on paper,
       empty repo skeleton with stub docs. NOT allowed: any `.go` file
       with real logic.
5. [ ] No vendoring third-party source without STDLIB.md disclosure
6. [ ] Team size 1–4 — confirmed
7. [ ] Track clearly targeted — Track D, stated in `.zero-dep.toml`
8. [ ] Source public on GitHub, OSI-approved license, public by
       submission deadline
9. [ ] AI tool usage is fine — judged on whether the artifact holds up
       (README/STDLIB.md/empty manifest are the "receipts")

## Track D specific requirements
- [ ] Persists and retrieves correctly across process restarts (AOF
      replay — must be tested, not assumed)
- [ ] Documents durability & consistency guarantees, **including
      corners cut** (no cross-key atomicity, no clustering, etc.)
- [ ] Uses stdlib file/buffer/hashing primitives only
- [ ] Survives a basic crash/concurrent-access test (kill -9 mid-write,
      restart, verify no corruption or documented recovery behavior)

## Submission deliverables (all required)
- [ ] Public GitHub repo, OSI-approved license (MIT or Apache-2.0)
- [ ] Single-command build → runnable artifact (`make build` or
      `go build ./cmd/govalk`)
- [ ] Empty dependency manifest, verified at submission
- [ ] Dependency proof: `deps-proof.txt` — output of `go list -m all`
      (should show only the main module, no requires) or CI log
- [ ] `README.md` — what it does, how to run it, **honest limits**
- [ ] `STDLIB.md` — every stdlib-for-package substitution, with
      rationale (aim for 10+ to also hit the STDLIB Log bonus)
- [ ] 5-minute demo video — tool actually working + empty manifest
      shown on screen
- [ ] `.zero-dep.toml` — track letter (`D`), one-line pitch

## Directory layout (advisory, matches PLAN.md §3)
- [ ] README.md
- [ ] STDLIB.md
- [ ] Makefile / build script (one command → artifact)
- [ ] src (`cmd/`, `internal/`) — weekend code only
- [ ] tests (`tests/`) — proves edge cases, crash/restart, concurrency
- [ ] go.mod — empty deps
- [ ] deps-proof.txt
- [ ] .zero-dep.toml

## Bonus challenges (optional, pick based on time remaining)
- [ ] **Single File** (+5, hard) — only attempt after MVP is solid;
      do not sacrifice readability (readability is 25% of the score,
      the bonus is worth less)
- [ ] **Reproducible Build** (+5, hard) — pin Go version, strip
      timestamps/paths from build, publish hashes of two independent
      builds in README
- [ ] **Package Killer** (+3, medium, + separate $100 prize) — name the
      specific real package(s) killed (e.g. `go-redis`, an embedded
      cache lib) explicitly in STDLIB.md/README
- [ ] **STDLIB Log** (+3, medium) — ≥10 real, non-trivial substitutions,
      one-line rationale each (tracked in §4 of PLAN.md, fill in as built)

## Scoring awareness (don't over-invest past diminishing returns)
- Functionality & Usefulness — 35% (highest weight: MVP correctness
  over feature count)
- Zero-Dependency Craft — 30% (STDLIB.md depth/honesty matters as much
  as the code itself)
- Code Quality & Idiom — 25% (idiomatic Go, clear error handling,
  no cleverness for its own sake)
- Innovation — 10% (lowest weight — don't burn hours chasing this)

## What scores badly — avoid
- [ ] Not a hello-world/single-function toy (MVP scope in PLAN.md §2
      avoids this)
- [ ] No hidden dependency via shelling out to installed tools
- [ ] No undisclosed vendoring
- [ ] Not rolling our own crypto (n/a — Track D, but if we ever hash
      anything, use stdlib `crypto/*` correctly, don't invent)
- [ ] Not an "LLM dump" — README/STDLIB.md/defensible design must be
      genuinely written and accurate, not boilerplate
- [ ] Doesn't require a running third-party service (no external DB,
      no cloud API — this project is fully self-contained, good fit)

## Write-up side quest (separate, optional, $300 total)
- [ ] Deadline: Sep 8, 2026 (after main event)
- [ ] Blog post judged on insight, not follower count; tag Hackathon
      Raptors
- [ ] Decide post-hackathon whether to write this
