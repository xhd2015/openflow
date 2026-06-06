package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	lessflags "github.com/xhd2015/less-flags"
)

func main() {
	os.Exit(run())
}

func run() int {
	var keepTemp bool = true
	_, _ = lessflags.Bool("--keep-temp", &keepTemp).Parse(os.Args[1:])

	start := time.Now()
	success := false

	pass := func(label string) {
		fmt.Printf("  %-55s PASS\n", label)
	}
	fail := func(label string, detail string) {
		fmt.Printf("  %-55s FAIL\n", label)
		fmt.Printf("    %s\n", detail)
	}

	fmt.Println("=== RUN   TestOpenflowMinimalBugFix")

	dir, err := os.MkdirTemp("", "openflow-minimal")
	if err != nil {
		fail("temp dir", err.Error())
		return 1
	}
	fmt.Printf("temp dir created: %s\n", dir)
	defer func() {
		if !success && !keepTemp {
			_ = os.RemoveAll(dir)
			return
		}
		fmt.Printf("temp dir kept: %s\n", dir)
	}()

	// --- Step 0: Precondition — bug exists ---
	bugDir := filepath.Join(dir, "merge")
	if out, err := exec.Command("cp", "-r",
		filepath.Join("testdata", "merge-bug"), bugDir,
	).CombinedOutput(); err != nil {
		fail("Precondition: copy fixture", fmt.Sprintf("%s\n%s", err, out))
		return 1
	}

	modInit := exec.Command("go", "mod", "init", "merge")
	modInit.Dir = bugDir
	if out, err := modInit.CombinedOutput(); err != nil {
		fail("Precondition: go mod init merge", fmt.Sprintf("%s\n%s", err, out))
		return 1
	}
	pass("Precondition: go mod init merge")

	bugTest := exec.Command("go", "test", "./...")
	bugTest.Dir = bugDir
	_, bugErr := bugTest.CombinedOutput()
	if bugErr == nil {
		fail("Precondition: bug exists in merge.go", "expected go test to FAIL (bug present), but it PASSED")
		return 1
	}
	pass("Precondition: bug exists in merge.go")
	fmt.Printf("    go test failed as expected\n")

	// --- Stage 1: openflow create ---
	fmt.Println("  --- Stage 1: openflow create ---")

	openflowBin := filepath.Join(dir, "openflow")
	buildOut, buildErr := exec.Command("go", "build", "-o", openflowBin, "./cmd/openflow/").CombinedOutput()
	if buildErr != nil {
		fail("Build openflow binary", fmt.Sprintf("%s\n%s", buildErr, buildOut))
		printTiming(start)
		return 1
	}
	pass("Build openflow binary")

	genWorkflowFile := filepath.Join(dir, "fix-merge.openflow.go")
	createCmd := exec.Command(openflowBin, "create",
		"Fix MergeJSON in merge.go so nested maps are recursively merged instead of replaced",
		"--out", genWorkflowFile,
		"--dir", dir,
	)
	createCmd.Env = append(os.Environ(),
		"OPENFLOW_HOME="+filepath.Join(dir, ".openflow"),
	)
	fmt.Println("    [creating workflow...]")
	createCmd.Stdout = os.Stdout
	createCmd.Stderr = os.Stderr
	createErr := createCmd.Run()
	if createErr != nil {
		printTiming(start)
		return 1
	}
	pass("openflow create")

	info, statErr := os.Stat(genWorkflowFile)
	if statErr != nil {
		fail("openflow create: file exists", statErr.Error())
		printTiming(start)
		return 1
	}
	if info.Size() == 0 {
		fail("openflow create: file non-empty", "file is 0 bytes")
		printTiming(start)
		return 1
	}
	pass("openflow create: file non-empty")

	content, readErr := os.ReadFile(genWorkflowFile)
	if readErr != nil {
		fail("openflow create: readable", readErr.Error())
		printTiming(start)
		return 1
	}

	pass("openflow create: file exists")
	fmt.Printf("=====openflow======\n%s\n=====openflow======\n", string(content))

	if !strings.Contains(string(content), "Agent") && !strings.Contains(string(content), "openflow") {
		fail("openflow create: contains openflow primitive",
			"generated file does not reference Agent or openflow")
		printTiming(start)
		return 1
	}
	pass("openflow create: contains openflow primitive")

	fmt.Printf("    generated: %s (%d bytes)\n", genWorkflowFile, info.Size())

	// rename to .txt
	workflowFile := genWorkflowFile + ".txt"
	err = os.Rename(genWorkflowFile, workflowFile)
	if err != nil {
		fail("rename", fmt.Sprintf("%s -> %s", genWorkflowFile, workflowFile))
	}

	// --- Stage 2: openflow run ---
	fmt.Println("  --- Stage 2: openflow run ---")

	runCmd := exec.Command(openflowBin, "run",
		workflowFile,
		"--dir", bugDir,
	)
	runCmd.Env = append(os.Environ(),
		"OPENFLOW_HOME="+filepath.Join(dir, ".openflow"),
	)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	fmt.Println("    [running workflow...]")
	runErr := runCmd.Run()
	if runErr != nil {
		fail("openflow run", fmt.Sprintf("exit=%v", runErr))
		printTiming(start)
		return 1
	}
	pass("openflow run")
	if !keepTemp {
		_ = os.RemoveAll(filepath.Join(bugDir, ".openflow-sdk"))
		_ = os.RemoveAll(filepath.Join(dir, ".openflow-sdk"))
	}

	verifyTest := exec.Command("go", "-C", "merge", "test", "./...")
	verifyTest.Dir = bugDir
	verifyOut, verifyErr := verifyTest.CombinedOutput()
	if verifyErr != nil {
		fail("Stage 2: bug fixed (go test passes)",
			fmt.Sprintf("go test still failing:\n%s", indent(string(verifyOut), "      ")))
		printTiming(start)
		return 1
	}
	pass("Stage 2: bug fixed (go test passes)")

	// --- Done ---
	success = true
	fmt.Printf("--- PASS: TestOpenflowMinimalBugFix (%.1fs)\n", time.Since(start).Seconds())
	return 0
}

func printTiming(start time.Time) {
	fmt.Printf("--- FAIL: TestOpenflowMinimalBugFix (%.1fs)\n", time.Since(start).Seconds())
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
