package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TestInfo struct {
	Name         string
	ContractPath string
	ContractName string
}

type TestConfig struct {
	maxSeconds        int
	maxFuzzRuns       int
	maxInvariantRuns  int
	maxInvariantDepth int
	maxConcurrency    int
	matchPaths        string
}

type TestResult struct {
	TestInfo
	Message  string
	Error    error
	Duration time.Duration
}

func main() {
	config := parseFlags()

	log.Println("Finding tests to run...")
	tests, err := findTests(config.matchPaths)
	if err != nil {
		log.Fatalf("Error getting test names: %v", err)
	}

	if len(tests) == 0 {
		log.Println("No tests found matching the criteria")
		return
	}

	log.Println("Running heavy fuzz tests...")
	ok := true
	for result := range runTestGroup(tests, config) {
		if result.Error != nil {
			ok = false
			log.Printf("FAIL: %s (%s)\n%v\n%s\n", result.Name, result.ContractName, result.Error, result.Message)
		} else {
			log.Printf("PASS: %s (%s) [%s]\n", result.Name, result.ContractName, result.Message)
		}
	}

	if !ok {
		os.Exit(1)
	}
}

func parseFlags() TestConfig {
	maxSeconds := flag.Int("max-seconds", -1, "maximum seconds per test (required)")
	maxFuzzRuns := flag.Int("max-fuzz-runs", -1, "maximum number of fuzz runs per test (required)")
	maxInvariantRuns := flag.Int("max-invariant-runs", -1, "maximum number of invariant runs per test (required)")
	maxInvariantDepth := flag.Int("max-invariant-depth", -1, "maximum depth of invariant runs per test (required)")
	maxConcurrency := flag.Int("max-concurrency", runtime.NumCPU(), "maximum number of concurrent test processes")
	matchPaths := flag.String("match-path", "", "path pattern to match for test files")
	flag.Parse()

	if *maxSeconds <= 0 {
		log.Fatal("max-seconds must be set with a positive value")
	}
	if *maxFuzzRuns <= 0 {
		log.Fatal("max-fuzz-runs must be set with a positive value")
	}
	if *maxInvariantRuns <= 0 {
		log.Fatal("max-invariant-runs must be set with a positive value")
	}
	if *maxInvariantDepth <= 0 {
		log.Fatal("max-invariant-depth must be set with a positive value")
	}
	if *maxConcurrency <= 0 {
		log.Fatal("max-concurrency must be set with a positive value")
	}

	log.Printf("Running with config:")
	log.Printf("  max-seconds: %d", *maxSeconds)
	log.Printf("  max-fuzz-runs: %d", *maxFuzzRuns)
	log.Printf("  max-invariant-runs: %d", *maxInvariantRuns)
	log.Printf("  max-invariant-depth: %d", *maxInvariantDepth)
	log.Printf("  max-concurrency: %d", *maxConcurrency)

	return TestConfig{
		maxSeconds:        *maxSeconds,
		maxFuzzRuns:       *maxFuzzRuns,
		maxInvariantRuns:  *maxInvariantRuns,
		maxInvariantDepth: *maxInvariantDepth,
		matchPaths:        *matchPaths,
		maxConcurrency:    *maxConcurrency,
	}
}

func findTests(matchPaths string) ([]TestInfo, error) {
	args := []string{"test", "--fuzz-runs", "1"}
	if matchPaths != "" {
		args = append(args, "--match-path", matchPaths)
	}

	cmd := exec.Command("forge", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting command: %w", err)
	}

	var tests []TestInfo
	scanner := bufio.NewScanner(stdout)
	testPattern := regexp.MustCompile(`(?:testFuzz_|invariant_)[^\s(]+`)
	contractPattern := regexp.MustCompile(`Ran \d+ tests? for ([^\s]+)`)
	currentContract := ""

	for scanner.Scan() {
		line := scanner.Text()

		if matches := contractPattern.FindStringSubmatch(line); len(matches) > 1 {
			currentContract = matches[1]
			continue
		}

		if matches := testPattern.FindString(line); matches != "" {
			testName := matches
			if idx := strings.Index(testName, "("); idx != -1 {
				testName = testName[:idx]
			}

			contractParts := strings.Split(currentContract, ":")
			if len(contractParts) != 2 {
				return nil, fmt.Errorf("invalid contract format '%s': expected 'path:name'", currentContract)
			}
			contractPath := contractParts[0]
			contractName := contractParts[1]

			tests = append(tests, TestInfo{
				Name:         testName,
				ContractPath: contractPath,
				ContractName: contractName,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("error waiting for command: %w", err)
	}

	for _, test := range tests {
		log.Printf("Found test: %s\n", test.Name)
	}

	return tests, nil
}

func runTest(test TestInfo, config TestConfig, wg *sync.WaitGroup, results chan<- TestResult) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.maxSeconds)*time.Second)
	defer cancel()

	args := []string{
		"test",
		"--match-test", test.Name,
		"--match-contract", test.ContractName,
		"--match-path", test.ContractPath,
		"--fuzz-runs", strconv.Itoa(config.maxFuzzRuns),
		"--threads", "1",
		"--fail-fast",
	}

	cmd := exec.CommandContext(ctx, "forge", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("FOUNDRY_INVARIANT_RUNS=%d", config.maxInvariantRuns))
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("FOUNDRY_INVARIANT_DEPTH=%d", config.maxInvariantDepth))

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := TestResult{
		TestInfo: test,
		Duration: duration,
	}

	if err != nil {
		result.Message = string(output)
		result.Error = err
	} else if ctx.Err() == context.DeadlineExceeded {
		result.Message = "timeout without failure"
	} else {
		result.Message = "completed"
	}

	results <- result
}

func runTestGroup(tests []TestInfo, config TestConfig) <-chan TestResult {
	results := make(chan TestResult, len(tests))
	var wg sync.WaitGroup
	sem := make(chan struct{}, config.maxConcurrency)

	for _, test := range tests {
		wg.Add(1)
		go func(t TestInfo) {
			sem <- struct{}{}
			runTest(t, config, &wg, results)
			<-sem
		}(test)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}
