# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project state

Rootforge is an unattended, event-driven incident investigation system. The root module now has its first executable slice: HTTP incident intake, normalization, open-Incident deduplication, in-memory Case creation/update, and Case lookup. Most remaining packages are still boundary scaffolds.

The only working code is `heapdump/hprofx/` — a standalone, zero-dependency Java HPROF heap dump analyzer (~2.5k lines) extracted from a real RabbitMQ WorkPool OOM investigation. It is a separate Go module and will be integrated into the scaffold through `internal/analyzer/heapdump/`.

When adding the first real implementation to a scaffold package, keep the existing `doc.go` contract line and the package `README.md` in sync with what you build.

## Commands

Two modules are joined by `go.work` (`.` and `./heapdump/hprofx`). The Makefile targets run both; plain `go ...` at the root only covers the root module.

```bash
make fmt        # go fmt across both modules
make test       # go test ./... across both modules
make vet        # go vet across both modules

go test ./internal/evidence/...                 # one package tree
go test ./internal/evidence/ -run TestFooBar    # one test
go -C heapdump/hprofx test ./...                # hprofx only
```

The Makefile pins `GOCACHE` to `$(CURDIR)/.rootforge/cache/go-build`, which is gitignored. Exporting the same value keeps direct `go` invocations sharing that cache.

### hprofx

```bash
cd heapdump/hprofx
make            # static binary → ./hprofx
make test       # go vet + gofmt check (hprofx has no Go tests)
make dist       # cross-compile darwin/linux × arm64/amd64 → dist/
./hprofx <dump.hprof> report -o analysis.md
```

Its own README documents the command set (`summary`, `histo`, `holders`, `retained`, `threads`, `amqp`, `inspect`, `paths`, `report`) and the parsing design; read it before touching the analyzer.

## Architecture

The system is six vertical layers plus a cross-cutting control plane (identity, secrets, redaction, approval, audit, budget). Full definitions live in `docs/system-layers.zh-CN.md`; the repo-level ownership table is in `ARCHITECTURE.md`.

```
L1 Acquire     trigger intake + bounded, on-demand evidence retrieval
L2 Case        normalization, provenance, snapshots      (internal/casefile)
L3 Analyze     deterministic, repeatable findings        (internal/analyzer)
L4 Correlate   runtime → image → build → commit → code   (internal/correlation)
L5 Investigate ClayHarness service via Rootforge adapter (internal/harness)
L6 Forge & Act candidate fix, policy decision, action    (internal/{forge,policy,action})
```

Guiding rule: **the closer to production, the more deterministic the behavior and the smaller the privilege; the closer to mutation, the stricter the approval.**

### Dependency rules that must not be broken

Layers communicate only through named artifacts — `EvidenceBatch` → `Finding` → `ContextGraph`/Timeline → `RCA` → `ChangeProposal` → `PolicyDecision` → `ActionResult`. Beyond that:

1. An upper layer reads a lower layer's standard artifact; it never bypasses the lower boundary to widen its own privilege.
2. Raw evidence is immutable; findings, graphs, RCAs, and proposals are versioned.
3. Every derived artifact must reference the evidence it came from.
4. LLM output is inference by default. Only raw evidence or deterministic analyzer output counts as fact.
5. Moving from read-only investigation to a write action requires a separate Policy decision and execution by the Action Executor.

### ClayHarness boundary

ClayHarness is an independently developed Rust service. Rootforge adapts a scoped Incident Case to its application-neutral protocol and retains authority over Case state, tools, evidence, credentials, policy, audit, and actions. ClayHarness never receives infrastructure credentials or arbitrary production shell access. It may request a declared tool or propose an action; Rootforge independently authorizes and executes either operation.

### Process boundaries

Three executables with distinct permissions, which may share a process during early development but must keep separate contracts:

- `cmd/rootforged` — control plane: receives events, owns incident state, schedules ClayHarness runs, enforces policy, dispatches notifications.
- `cmd/rootforge` — administrative CLI (config checks, Case inspection, offline import, replay). **Not** the normal investigation entry point; events are.
- `cmd/rootforge-retriever` — optional low-privilege, read-only node retriever. Never hosts an LLM or an arbitrary shell endpoint.

### Evidence acquisition

`internal/evidence` queries existing source systems (Loki, Elasticsearch, Docker, journald) or an optional retriever on demand, per Case, within a budget. It is explicitly **not** a log platform: no continuous ingestion, long-term storage, full-text index, search UI, or telemetry forwarding. Every query carries time, entity, record-count, byte, and timeout bounds, and must report `Truncated` rather than silently return partial data as if complete. Source failures produce partial results with explicit warnings. See `docs/evidence-acquisition.zh-CN.md` for the unified `EvidenceQuery`/`LogRecord`/`EvidenceBatch` contracts and the intended implementation order (file → docker/journald → loki → elasticsearch → Case-scoped follow).

### v0.1 scope

One narrow vertical slice: a JVM/Docker OOM event auto-opens a Case → `hprofx` produces structured findings → manifest maps service/image/repo/commit → a traceable RCA is generated → the developer is notified. No production mutation in v0.1; write actions default to deny at every automation level.

## Conventions

- Repo-facing documents (`README.md`, `ARCHITECTURE.md`, package READMEs, ADRs, Go doc comments) are in English. Product and layer design docs under `docs/` are in Chinese (`*.zh-CN.md`).
- Significant, hard-to-reverse decisions get an ADR in `docs/adr/` (context → decision → consequences); older ADRs are superseded, not silently rewritten.
- Never commit production logs, credentials, source archives, heap dumps, or unredacted Incident Cases. Fixtures under `testdata/cases/` must be synthetic or explicitly sanitized. `.gitignore` already excludes `*.hprof`, `/cases/`, and `/.rootforge/`.
- No license has been chosen yet — do not add license headers or a LICENSE file without being asked.
