package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type TestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

type TestCase struct {
	Name    string
	Package string
	Elapsed time.Duration
	Outputs []string
}

type PackageResult struct {
	Name         string
	Total        int
	Passed       []TestCase
	Failed       []TestCase
	Skipped      []TestCase
	Action       string
	Elapsed      time.Duration
	currentTest  string
	OutputBuffer map[string][]string
}

type TestRunSummary struct {
	Packages    map[string]*PackageResult
	StartTime   time.Time
	EndTime     time.Time
	BuildErrors []string
}

var fileLineRe = regexp.MustCompile(`(\w+\.go:\d+):`)

func main() {
	args := []string{"test", "-json", "-count=1"}
	hasPkgPattern := false
	for _, a := range os.Args[1:] {
		if !strings.HasPrefix(a, "-") {
			hasPkgPattern = true
		}
	}
	if len(os.Args) > 1 {
		args = append(args, os.Args[1:]...)
	}
	if !hasPkgPattern {
		args = append(args, "./...")
	}

	summary := &TestRunSummary{
		Packages:  make(map[string]*PackageResult),
		StartTime: time.Now(),
	}

	cmd := exec.Command("go", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to create stdout pipe: %v\n", err)
		os.Exit(1)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to start go test: %v\n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		var event TestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			fmt.Println(line)
			continue
		}

		if event.Action == "output" {
			fmt.Print(event.Output)
		}
		processEvent(summary, event)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: scanner error: %v\n", err)
	}

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	summary.EndTime = time.Now()

	printSummary(summary, exitCode)
	os.Exit(exitCode)
}

func processEvent(summary *TestRunSummary, event TestEvent) {
	pkgName := event.Package
	if pkgName == "" {
		pkgName = "(unknown)"
	}

	pkg, ok := summary.Packages[pkgName]
	if !ok {
		pkg = &PackageResult{Name: pkgName, OutputBuffer: make(map[string][]string)}
		summary.Packages[pkgName] = pkg
	}

	if event.Test == "" {
		switch event.Action {
		case "pass":
			pkg.Action = "pass"
			pkg.Elapsed = durationFromSeconds(event.Elapsed)
		case "fail":
			pkg.Action = "fail"
			pkg.Elapsed = durationFromSeconds(event.Elapsed)
		case "skip":
			pkg.Action = "skip"
		case "output":
			if strings.Contains(event.Output, "FAIL") {
				pkg.Action = "fail"
			}
		}
		return
	}

	switch event.Action {
	case "run":
		pkg.currentTest = event.Test
		pkg.Total++
	case "pass":
		tc := TestCase{
			Name:    event.Test,
			Package: pkgName,
			Elapsed: durationFromSeconds(event.Elapsed),
		}
		if buf, ok := pkg.OutputBuffer[event.Test]; ok {
			tc.Outputs = buf
			delete(pkg.OutputBuffer, event.Test)
		}
		pkg.Passed = append(pkg.Passed, tc)
	case "fail":
		tc := TestCase{
			Name:    event.Test,
			Package: pkgName,
			Elapsed: durationFromSeconds(event.Elapsed),
		}
		if buf, ok := pkg.OutputBuffer[event.Test]; ok {
			tc.Outputs = buf
			delete(pkg.OutputBuffer, event.Test)
		}
		pkg.Failed = append(pkg.Failed, tc)
	case "skip":
		tc := TestCase{
			Name:    event.Test,
			Package: pkgName,
			Elapsed: durationFromSeconds(event.Elapsed),
		}
		if buf, ok := pkg.OutputBuffer[event.Test]; ok {
			tc.Outputs = buf
			delete(pkg.OutputBuffer, event.Test)
		}
		pkg.Skipped = append(pkg.Skipped, tc)
	case "output":
		pkg.OutputBuffer[event.Test] = append(pkg.OutputBuffer[event.Test], event.Output)
	}
}

func durationFromSeconds(sec float64) time.Duration {
	return time.Duration(sec*1000) * time.Millisecond
}

func printSummary(summary *TestRunSummary, exitCode int) {
	total := 0
	passed := 0
	failed := 0
	skipped := 0
	var allFailed []TestCase
	var allSkipped []TestCase
	totalElapsed := summary.EndTime.Sub(summary.StartTime)

	for _, pkg := range summary.Packages {
		total += pkg.Total
		passed += len(pkg.Passed)
		failed += len(pkg.Failed)
		skipped += len(pkg.Skipped)
		allFailed = append(allFailed, pkg.Failed...)
		allSkipped = append(allSkipped, pkg.Skipped...)
	}

	resultText := "PASS"
	if exitCode != 0 {
		resultText = "FAIL"
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  TEST SUMMARY\n")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  Total   : %6d tests\n", total)
	fmt.Printf("  Passed  : %6d\n", passed)
	fmt.Printf("  Failed  : %6d\n", failed)
	fmt.Printf("  Skipped : %6d\n", skipped)
	fmt.Printf("  Time    : %7.3fs\n", totalElapsed.Seconds())
	fmt.Printf("  Result  :  %s\n", resultText)

	if len(allFailed) > 0 {
		fmt.Println()
		fmt.Println(strings.Repeat("\u2500", 60))
		fmt.Println("  FAILED TESTS")
		fmt.Println(strings.Repeat("\u2500", 60))
		for i, tc := range allFailed {
			fileRef := extractFileRef(tc.Outputs)
			fmt.Printf("  %d) %s\n", i+1, tc.Name)
			if fileRef != "" {
				fmt.Printf("     File    : %s\n", fileRef)
			}
			if len(tc.Outputs) > 0 {
				msg := extractErrorMessage(tc.Outputs)
				if msg != "" {
					fmt.Printf("     Message : %s\n", msg)
				}
			}
			fmt.Println()
		}
	}

	if len(allSkipped) > 0 {
		fmt.Println()
		fmt.Println(strings.Repeat("\u2500", 60))
		fmt.Println("  SKIPPED TESTS")
		fmt.Println(strings.Repeat("\u2500", 60))
		for i, tc := range allSkipped {
			fileRef := extractFileRef(tc.Outputs)
			fmt.Printf("  %d) %s\n", i+1, tc.Name)
			if fileRef != "" {
				fmt.Printf("     File    : %s\n", fileRef)
			}
			if len(tc.Outputs) > 0 {
				msg := extractSkipMessage(tc.Outputs)
				if msg != "" {
					fmt.Printf("     Message : %s\n", msg)
				}
			}
			fmt.Println()
		}
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  RESULT: %s (exit code %d)\n", resultText, exitCode)
	fmt.Println(strings.Repeat("=", 60))
}

func extractFileRef(outputs []string) string {
	for _, line := range outputs {
		matches := fileLineRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			ref := strings.TrimSpace(matches[1])
			if ref != "" {
				ref = filepath.ToSlash(ref)
				prefixes := []string{"internal/", "tests/"}
				for _, p := range prefixes {
					if idx := strings.Index(ref, p); idx >= 0 {
						return ref[idx:]
					}
				}
				return ref
			}
		}
	}
	return ""
}

func extractSkipMessage(outputs []string) string {
	for _, line := range outputs {
		trimmed := strings.TrimSpace(line)
		if idx := fileLineRe.FindStringIndex(trimmed); idx != nil {
			after := strings.TrimSpace(trimmed[idx[1]:])
			if after != "" {
				return after
			}
		}
	}
	for _, line := range outputs {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "=== RUN") && !strings.HasPrefix(trimmed, "--- SKIP") {
			return trimmed
		}
	}
	return ""
}

func extractErrorMessage(outputs []string) string {
	var msgs []string
	for _, line := range outputs {
		trimmed := strings.TrimSpace(line)
		if idx := fileLineRe.FindStringIndex(trimmed); idx != nil {
			after := strings.TrimSpace(trimmed[idx[1]:])
			if after != "" {
				msgs = append(msgs, after)
			}
		}
	}
	if len(msgs) > 0 {
		return strings.Join(msgs, "\n              ")
	}
	for _, line := range outputs {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
