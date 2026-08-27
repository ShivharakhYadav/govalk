# Project Plan — "govalk" (working name)

A Redis-protocol-compatible, in-memory key-value store with crash-safe
persistence, written in Go, zero third-party runtime dependencies.

Target: **Zero Dependency Hackathon**, Track D (Data & Storage).
Secondary alignment: Track C (Web & Network) — real TCP protocol server.

---

## 1. Why this project (recap of the bet)

- Functionality (35%): demoable *live* against the real `redis-cli` —
  no need to trust a screenshot, judges can verify correctness themselves.
- Zero-dep craft (30%): every layer (protocol parsing, persistence,
  concurrency, TTL sweep) is something Go usually pulls a package for —
  each one becomes a documented STDLIB.md entry.
- Code quality (25%): small, well-understood domain (RESP is a simple
  wire format) means we can afford idiomatic, well-tested code instead
  of racing to make something merely functional.
- Bonus targets: Package Killer (kills `go-redis`/embedded cache libs),
  Reproducible Build (Go makes this close to free), Single File (stretch
  goal, only if time allows — don't sacrifice readability for it),
  STDLIB Log (natural byproduct of the architecture, aim for 10+ entries).

## 2. Scope (MVP — must have by hour ~48, polish after)

Commands (RESP protocol, subset):
- `PING`, `ECHO`
- `SET key value [EX seconds]`, `GET key`, `DEL key`, `EXISTS key`
- `EXPIRE key seconds`, `TTL key`, `PERSIST key`
- `KEYS pattern` (basic glob, stdlib `path/filepath.Match` or hand-rolled)
- One extra structure: `LPUSH`/`RPUSH`/`LRANGE` (list) — pick this over
  hashes, it's a better demo and exercises encoding harder.

Server:
- TCP listener, one goroutine per connection, RESP request/response
  parser (hand-written, not reflection-based).
- Graceful shutdown (SIGINT/SIGTERM) that flushes persistence before exit.

Persistence:
- Append-only file (AOF): every write command serialized and appended.
- On startup, replay AOF to rebuild in-memory state.
- Periodic (or size-triggered) compaction: rewrite AOF from current
  in-memory snapshot to bound file growth — this is the "durability
  guarantee" writeup for the README/STDLIB.md.

Concurrency model:
- Sharded store (N buckets, each with its own `sync.RWMutex`) keyed by
  hash of the key — avoids one global lock, documents the consistency
  tradeoff (no cross-key atomicity) honestly per Track D requirements.
- Background goroutine for lazy TTL expiry sweep (ticker-based),
  plus lazy expiry-on-read as a fallback.

Out of scope (document as cut corners in README, do not silently skip):
- Clustering/replication
- RDB-style binary snapshot format (AOF only, simpler + easier to audit)
- Pub/Sub, transactions (MULTI/EXEC), Lua scripting
- Auth/ACLs (no `AUTH` — flag as a threat-model note, not a Track E tool)

## 3. Package layout (planned, not yet created as code)

```
govalk/
├── README.md              # what/why/how, honest limits
├── STDLIB.md               # every stdlib-for-package substitution
├── .zero-dep.toml           # track: D, one-line pitch
├── Makefile                 # single-command build -> ./bin/govalk
├── go.mod                   # empty require block
├── deps-proof.txt            # `go list -m all` output at submission time
├── cmd/
│   └── govalk/
│       └── main.go           # flag parsing (stdlib `flag`), wiring
├── internal/
│   ├── resp/                 # RESP protocol encode/decode
│   ├── store/                 # sharded in-memory KV + TTL engine
│   ├── persist/                # AOF writer/reader + compaction
│   └── server/                  # TCP listener, connection handling
└── tests/
    ├── resp_test.go
    ├── store_test.go
    ├── persist_test.go            # crash/restart correctness
    └── integration_test.go         # spins up server, talks RESP over TCP
```

## 4. Stdlib substitution targets (populate STDLIB.md as built)

| Normally reach for...          | Stdlib replacement plan                       |
|---------------------------------|------------------------------------------------|
| `go-redis` client (for testing) | Hand-rolled RESP client in integration tests    |
| `github.com/tidwall/redcon`     | Hand-written TCP + RESP parser (`net`, `bufio`) |
| `patrickmn/go-cache`            | Custom sharded map + TTL sweep (`sync`, `time`) |
| `spf13/cobra` / `urfave/cli`    | Stdlib `flag` package                            |
| `sirupsen/logrus` / `zap`       | Stdlib `log/slog`                                 |
| `stretchr/testify`              | Stdlib `testing` + table-driven tests             |
| `google/uuid`                   | `crypto/rand` + manual hex/UUID formatting (if needed) |
| gob/protobuf for AOF encoding   | Hand-rolled length-prefixed binary format (`encoding/binary`) |

(Final list goes in STDLIB.md with rationale per entry — aim for 10+ to
hit the STDLIB Log bonus.)

## 5. Timeline (72h window: Aug 28 18:00 UTC → Aug 31 18:00 UTC)

**Note:** per Rule #4, none of this code gets written before kickoff.
Everything below Aug 28 18:00 UTC is planning/design only (allowed).

- **Now → Aug 28 kickoff:** finalize RESP command subset, AOF binary
  format spec, sharding scheme on paper; set up empty repo skeleton
  (README/STDLIB.md/.zero-dep.toml stubs, empty go.mod) — no logic.
- **Hour 0–8:** RESP parser + encoder, unit tests. Bare TCP echo server.
- **Hour 8–16:** In-memory sharded store + GET/SET/DEL/EXISTS wired to
  RESP dispatch. Manual test via `redis-cli`.
- **Hour 16–24:** TTL (EXPIRE/TTL/PERSIST) + background sweep.
- **Hour 24–32:** AOF persistence (write path) + startup replay.
- **Hour 32–40:** LPUSH/RPUSH/LRANGE. AOF compaction.
- **Hour 40–48:** Crash/restart tests, concurrency stress test, KEYS glob.
  **MVP feature-complete by here.**
- **Hour 48–56:** README + STDLIB.md written for real (not stubs).
  Graceful shutdown, error handling pass, idiom cleanup pass.
- **Hour 56–64:** Bonus challenge pass: reproducible build (pin Go
  version, strip build-time paths/timestamps, publish hashes),
  attempt single-file if it doesn't hurt readability.
- **Hour 64–70:** Record 5-minute demo video (live redis-cli session +
  kill process + restart to show AOF replay).
- **Hour 70–72:** Buffer for submission mechanics (deps-proof.txt,
  final repo cleanup, .zero-dep.toml, double-check public repo + license).

## 6. Risks / mitigations

- **RESP edge cases (bulk strings, inline commands) eating time** →
  scope to RESP2 only, skip RESP3, document the cut.
- **AOF compaction bugs corrupting data** → write compaction to a temp
  file + atomic rename, never mutate the live AOF in place.
- **Concurrency bugs under stress test** → run `go test -race` from
  hour 1, not as an afterthought.
- **Running out of time for polish** → MVP checkpoint at hour 48 is a
  hard gate; anything not done by then gets cut and documented, not
  rushed.
