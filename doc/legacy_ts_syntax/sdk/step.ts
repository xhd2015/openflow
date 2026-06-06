import { emitEvent, log } from "./events";

export async function step(label: string, fn: () => Promise<void>): Promise<void> {
  const ts = new Date().toISOString();
  log("step", label);
  emitEvent({ type: "step", step: label, status: "started", timestamp: ts });

  try {
    await fn();
    const doneTs = new Date().toISOString();
    log("step", `${label} done`);
    emitEvent({ type: "step", step: label, status: "completed", timestamp: doneTs });
  } catch (e: any) {
    const failTs = new Date().toISOString();
    log("step", `${label} failed: ${String(e)}`);
    emitEvent({ type: "step", step: label, status: "failed", timestamp: failTs, error: String(e) });
    throw e;
  }
}
