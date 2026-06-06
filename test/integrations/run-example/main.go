package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	lessflags "github.com/xhd2015/less-flags"
)

func main() {
	os.Exit(run())
}

func run() int {
	var keepTemp bool
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

	fmt.Println("=== RUN   TestOpenflowRunExample")

	dir, err := os.MkdirTemp("", "openflow-run-example")
	if err != nil {
		fail("temp dir", err.Error())
		return 1
	}
	defer func() {
		if !success && !keepTemp {
			_ = os.RemoveAll(dir)
			return
		}
		fmt.Printf("temp dir kept: %s\n", dir)
	}()

	exampleSrc := filepath.Join("test", "integrations", "run-example", "example.openflow.go.txt")
	exampleDst := filepath.Join(dir, "example.openflow.go.txt")
	if out, err := exec.Command("cp", exampleSrc, exampleDst).CombinedOutput(); err != nil {
		fail("copy example", fmt.Sprintf("%s\n%s", err, out))
		printTiming(start)
		return 1
	}

	openflowBin := filepath.Join(dir, "openflow")
	buildOut, buildErr := exec.Command("go", "build", "-o", openflowBin, "./cmd/openflow/").CombinedOutput()
	if buildErr != nil {
		fail("Build openflow binary", fmt.Sprintf("%s\n%s", buildErr, buildOut))
		printTiming(start)
		return 1
	}
	pass("Build openflow binary")

	fmt.Println("  --- openflow run ---")
	fmt.Println("    [running workflow...]")

	runCmd := exec.Command(openflowBin, "run",
		exampleDst,
		"--dir", dir,
	)
	runCmd.Env = append(os.Environ(),
		"OPENFLOW_HOME="+filepath.Join(dir, ".openflow"),
	)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runErr := runCmd.Run()
	if runErr != nil {
		fail("openflow run", fmt.Sprintf("exit=%v", runErr))
		printTiming(start)
		return 1
	}
	pass("openflow run exited cleanly")

	success = true
	fmt.Printf("--- PASS: TestOpenflowRunExample (%.1fs)\n", time.Since(start).Seconds())
	return 0
}

func printTiming(start time.Time) {
	fmt.Printf("--- FAIL: TestOpenflowRunExample (%.1fs)\n", time.Since(start).Seconds())
}
