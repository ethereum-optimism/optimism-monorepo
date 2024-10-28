package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/ethereum-optimism/optimism/op-program/prestates"
)

func main() {
	var inputFile string
	flag.StringVar(&inputFile, "input", "", "Releases JSON file to verify")
	flag.Parse()
	if inputFile == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Must specify --input")
		os.Exit(2)
	}

	in, err := os.OpenFile(inputFile, os.O_RDONLY, 0o644)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to open input file: %v\n", err.Error())
		os.Exit(2)
	}
	defer in.Close()

	input, err := os.ReadFile(inputFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to read input file: %v\n", err.Error())
		os.Exit(2)
	}
	var actual []prestates.Release
	err = json.Unmarshal(input, &actual)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to parse JSON: %v\n", err.Error())
		os.Exit(2)
	}

	expected, err := prestates.GetReleases()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to load expected releases: %v\n", err.Error())
		os.Exit(2)
	}

	sortFunc := func(a, b prestates.Release) int {
		if a.Version > b.Version {
			return 1
		} else if a.Version == b.Version {
			return 0
		}
		return -1
	}
	slices.SortFunc(actual, sortFunc)
	slices.SortFunc(expected, sortFunc)

	differs := false
	report := ""
	for i := 0; i < max(len(actual), len(expected)); i++ {
		get := func(arr []prestates.Release, idx int) string {
			if i >= len(arr) {
				return "<missing>"
			} else {
				return formatRelease(arr[i])
			}
		}
		expectedStr := get(expected, i)
		actualStr := get(actual, i)
		releaseDiffers := expectedStr != actualStr
		marker := "✅"
		if releaseDiffers {
			marker = "❌"
		}
		report += fmt.Sprintf("%v %d\tExpected: %v\tActual: %v\n", marker, i, expectedStr, actualStr)
		differs = differs || releaseDiffers
	}
	fmt.Println(report)
	if differs {
		os.Exit(1)
	}
}

func formatRelease(release prestates.Release) string {
	return fmt.Sprintf("%-13v %s", release.Version, release.Hash)
}
