import { Agent, shell, step } from "openflow";

const coder = new Agent({
  name: "coder",
  systemPrompt: "You are a Go programmer. Fix bugs in Go code carefully. When fixing recursive merge logic, ensure non-map values still get replaced and only map+map pairs recurse. Preserve existing behavior for all non-map cases.",
});

let feedback = "";
let done = false;
let i = 0;
const MAX = 5;

while (!done && i < MAX) {
  i++;
  await step("ITERATION " + i, async () => {

    await step("FIX MERGE", () =>
      coder.run(
        "Fix MergeJSON in merge.go so nested maps are recursively merged instead of replaced. Look at the file in the current directory. The function should: (1) if both values at a key are map[string]interface{}, recursively merge them; (2) otherwise the new value replaces the old; (3) handle nil inputs gracefully. Run `go test ./...` after your change to verify.",
        { feedback }
      )
    );

    feedback = "";

    await step("VERIFY", async () => {
      const r = await shell("go test ./...");
      if (r.exitCode === 0) {
        done = true;
        return;
      }
      feedback = r.feedback;
    });

  });
}

console.log(done ? "FIXED" : "GAVE UP");
