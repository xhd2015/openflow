# Openflow CLI — High-Level Design

## Overview

`openflow` is a **standalone CLI** (separate from murphy/codenn) that executes `.openflow.ts` files.
A `.openflow.ts` file orchestrates LLM agents and shell commands in TypeScript loops, enabling
fully-automated multi-step engineering tasks (e.g. "debug CI until green").

The `openflow` CLI reuses `github.com/xhd2015/agent-pro` for agent invocation but is otherwise
independent of murphy/codenn.

## Lifecycle

```
User describes goal
  │
  ├─ Path A: openflow create "description" [--out file] [--run]
  │    CLI calls agent-pro to generate .openflow.ts, optionally runs it
  │
  └─ Path B: any agent (codenn, codex, etc.) writes .openflow.ts
       Agent reads OPENFLOW.md for SDK reference, generates file,
       can later re-edit and re-run iteratively
  │
  ▼
openflow run <file>
  │
  ├─ Validate: tsc --noEmit (optional, --skip-validate to bypass)
  ├─ Set up env vars (OPENFLOW_CONFIG_HOME, OPENFLOW_TRACE_ID, ...)
  ├─ Resolve SDK import path
  └─ Execute: bun run <file>
       │
       └─ User's .openflow.ts uses:
            Agent.run()     → spawns agent-pro runner subprocess
            shell()         → runs shell commands, captures output
            step()          → emits trace event for visualization
  │
  ▼
openflow status     → terminal summary
openflow trace      → web viewer (reuses agent-pro trace viewer)
```

## Architecture

```
┌─────────────────────────────────────────┐
│  cmd/openflow/main.go                   │
│  CLI entry: run, create, status, trace  │
└───────────┬─────────────────────────────┘
            │
┌───────────▼─────────────────────────────┐
│  internal/                              │
│    runner/   - validate, invoke bun,    │
│               set up env, capture trace │
│    creator/  - call agent-pro to        │
│               generate .openflow.ts     │
│    trace/    - write openflow-level     │
│               trace events              │
└───────────┬─────────────────────────────┘
            │
┌───────────▼─────────────────────────────┐
│  sdk/       (embedded via //go:embed)   │
│    index.ts - export { Agent, shell,    │
│               step }                    │
│    agent.ts - Agent class: spawns       │
│               agent-pro runner binary   │
│    shell.ts - spawn shell commands      │
│    step.ts  - trace checkpoint emitter  │
└─────────────────────────────────────────┘
```

## Dependencies

- `github.com/xhd2015/agent-pro` — agent runner invocation (codex, opencode, cursor)
- `bun` — TypeScript runtime for `.openflow.ts` files
- `tsc` (optional) — TypeScript validation

## Relationship to murphy/codenn

- `openflow` is a **separate Go module**, not a subcommand of murphy
- Shares `agent-pro` as the common agent invocation layer
- Openflow traces are stored under `~/.openflow/` (separate from `~/.murphy-codenn/`)
- An agent (codenn/murphy) can create and run openflows by calling the `openflow` binary

## Storage Layout

```
~/.openflow/
  agent-traces/              # agent-pro compatible trace sessions
    <trace-id>/
      metadata.json
      events.jsonl
      prompt.md

  runs/                      # openflow run records
    <timestamp>-<run-id>/
      metadata.json          # {id, file, status, started_at, finished_at, error}
      steps.jsonl            # one JSON line per step() call
      agent-calls.jsonl      # one JSON line per agent.run() call
```

## Data Models

### Run Metadata (`runs/<id>/metadata.json`)
```json
{
  "id": "20260605-abc123",
  "openflow_file": "/abs/path/to/ci-debug.openflow.ts",
  "status": "running|completed|failed",
  "started_at": "2026-06-05T10:00:00Z",
  "finished_at": null,
  "error": null,
  "exit_code": null,
  "trace_id": "abc123"
}
```

### Step Event (`runs/<id>/steps.jsonl`)
```json
{"timestamp":"...","step":"CODE CHANGE","status":"started"}
{"timestamp":"...","step":"CODE CHANGE","status":"completed","duration_ms":1234}
```

### Agent Call Event (`runs/<id>/agent-calls.jsonl`)
```json
{"timestamp":"...","agent":"coder","prompt":"fix code","status":"running"}
{"timestamp":"...","agent":"coder","status":"completed","output":"...","codenn_session":"..."}
```

## SDK API Surface

```typescript
// sdk/index.ts

interface AgentOptions {
  systemPrompt: string;
  agentRunner?: string;   // "codex" | "opencode" | "cursor" (default: "opencode")
  model?: string;         // model override
}

class Agent {
  constructor(opts: AgentOptions);
  run(prompt: string): Promise<string>;
  runAsync(prompt: string): Promise<string>;
}

interface ShellResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

function shell(cmd: string, opts?: { cwd?: string; env?: Record<string,string> }): Promise<ShellResult>;

function step(label: string, fn: () => Promise<void>): Promise<void>;
```

## Key Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Standalone CLI, not murphy subcommand | Openflow orchestrates agents generically; no murphy semantics needed |
| 2 | TypeScript as openflow language | Full programming language; loops, conditionals, async — free |
| 3 | `bun` as runtime | Fast startup, native TS, good child_process support |
| 4 | Embedded SDK via `//go:embed` | No npm install step; self-contained binary |
| 5 | `agent.run()` shells to codenn/agent-pro binary | Reuses existing agent infrastructure without JS-native SDK |
| 6 | Trace events as JSONL files | Compatible with agent-pro trace viewer |
| 7 | `OPENFLOW.md` for agent-driven creation | Same pattern as CODENN.md/MURPHY.md — agents read instruction file |
