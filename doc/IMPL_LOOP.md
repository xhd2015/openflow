# Openflow Implementation Loop Plan

## Goal

Build the `openflow` CLI until the minimal integration test passes.  
The test exercises `openflow create` + `openflow run` end-to-end, using real opencode, to fix a real bug.

## The Loop

**I (the assistant) am the executor.** No shell script, no codenn. Each iteration:

```
go run ./test/integrations/minimal/          ← step 3
  │
  ├─ exit 0 → DONE (loop ends)
  └─ exit 1 → I read output, analyse failures, edit code → repeat
```

Steps per iteration:
1. Analyse the test output / build errors
2. Edit code files to fix the failures
3. Run `go run ./test/integrations/minimal/`
4. Check output — if pass, done; if fail, go to step 1

## Dependencies

- `github.com/xhd2015/agent-pro` (only external dep; replaced locally to `../agent-pro`)
- `bun` — TypeScript runtime for `.openflow.ts` files (required on PATH)
- `opencode` — agent runner (required on PATH)
- No dependency on murphy or codenn

## Directory Layout

```
external/openflow/
  go.mod                              # module github.com/xhd2015/openflow
  IMPL_LOOP.md                        # this file

  cmd/openflow/main.go                # CLI entry: create, run, status, trace
  internal/
    creator/creator.go                # calls agent-pro to generate .openflow.ts
    runner/runner.go                  # extracts SDK, spawns bun, manages traces
  sdk/                                # embedded via //go:embed in Go binary
    index.ts                          # export { Agent, shell, step }
    agent.ts                          # Agent class → spawns opencode via agent-pro
    shell.ts                          # shell(cmd) → Bun.spawn, captures output
    step.ts                           # step(label, fn) → trace checkpoint

  testdata/merge-bug/
    merge.go                          # MergeJSON(base,override) — BUG: replaces nested maps
    merge_test.go                     # go test — FAILS because of bug

  test/integrations/minimal/
    main.go                           # THE MINIMAL TEST
```

## The Minimal Test (`test/integrations/minimal/main.go`)

Standard `main.go` — NOT a `_test.go` file. Runnable with `go run`.

Output style mimics `go test` output (=== RUN / --- PASS / --- FAIL / PASS / FAIL lines).

### Stages

| Stage | Action | Verification |
|-------|--------|-------------|
| 0 - Precondition | Copy `testdata/merge-bug/` to a temp dir, run `go test` there | `go test` FAILS (bug confirmed present) |
| 1 - Create | `openflow create "Fix MergeJSON" --out fix.openflow.ts --dir <tmp>` | File exists > 0 bytes, valid TS (`tsc --noEmit`), contains `Agent` or `from "openflow"` |
| 2 - Run | `openflow run fix.openflow.ts --dir <tmp>` | run `go test` in temp dir → PASSES (bug fixed) |

### Key details

- `opencode` resolved via `PATH`; test skips (`t.Skip`-style exit) if not found
- Test timeout: 10 minutes (`go run -timeout 10m` or built-in deadline)
- `openflow create` delegates entirely to agent-pro (shells to opencode)
- `openflow run` delegates `agent.run()` entirely to agent-pro (shells to opencode)
- SDK files embedded via `//go:embed`; extracted to `OPENFLOW_HOME/sdk/` before bun runs
- `OPENFLOW_HOME` set to temp dir by the test for isolation
- Openflow traces written under `OPENFLOW_HOME` (temp dir)

### Expected first-run output

```
=== RUN   TestOpenflowMinimalBugFix
  Precondition: bug exists in merge.go ............ PASS
  Stage 1: openflow create ........................ FAIL
    openflow binary not found (build failed)
=== FAIL: TestOpenflowMinimalBugFix (0.1s)
exit status 1
```

### Expected final output (after all implementations)

```
=== RUN   TestOpenflowMinimalBugFix
  Precondition: bug exists in merge.go ............ PASS
  Stage 1: openflow create ........................ PASS
    file exists ................................... PASS
    valid TypeScript .............................. PASS
    contains openflow primitive ................... PASS
  Stage 2: openflow run ........................... PASS
    bug fixed (go test passes) .................... PASS
=== PASS: TestOpenflowMinimalBugFix (45.2s)
```

## Data Models

Implementation uses the models from DESIGN.md.

## Bootstrapping Order

The loop will naturally drive this order (failing at each stage until implemented):

1. `cmd/openflow/main.go` — CLI entry, `go build` must succeed
2. `internal/creator/` — `openflow create` must generate a file
3. `sdk/*.ts` — embedded SDK, must be extracted before bun runs
4. `internal/runner/` — `openflow run` must extract SDK, spawn bun, capture output
5. Polish — error messages, edge cases, exit codes
