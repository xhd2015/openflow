import { emitEvent, log } from "./events";

export interface ShellOptions {
  cwd?: string;
  env?: Record<string, string>;
  timeout?: number;
}

export interface ShellResult {
  stdout: string;
  stderr: string;
  exitCode: number;
  feedback: string;
}

export async function shell(cmd: string, opts?: ShellOptions): Promise<ShellResult> {
  const startMs = Date.now();

  const proc = Bun.spawn({
    cmd: ["sh", "-c", cmd],
    cwd: opts?.cwd || undefined,
    env: opts?.env || process.env,
    stdout: "pipe",
    stderr: "pipe",
  });

  const timeout = opts?.timeout || 300000;
  const timer = setTimeout(() => {
    proc.kill();
  }, timeout);

  const stdout = await new Response(proc.stdout).text();
  const stderr = await new Response(proc.stderr).text();
  const exitCode = await proc.exited;
  clearTimeout(timer);

  const durationMs = Date.now() - startMs;
  const ts = new Date().toISOString();

  if (exitCode !== 0) {
    log("shell", `${truncate(cmd, 80)} FAILED: ${stderr.trim() || `exit ${exitCode}`}`);
  } else {
    log("shell", `${truncate(cmd, 80)} → exit 0 (${durationMs}ms)`);
  }

  emitEvent({
    type: "shell",
    cmd,
    exit_code: exitCode,
    stdout,
    stderr,
    duration_ms: durationMs,
    timestamp: ts,
  });

  return {
    stdout,
    stderr,
    exitCode,
    feedback: `${truncate(cmd, 200)}
exit code: ${exitCode}
${stdout}${stderr}`.trim(),
  };
}

function truncate(s: string, max: number): string {
  s = s.replace(/\s+/g, " ").trim();
  if (s.length <= max) return s;
  return s.slice(0, max - 3) + "...";
}
