package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl/fake"
)

func main() {
	// Parse command line flags
	templateFile := flag.String("template", "", "Path to template file")
	dataFile := flag.String("data", "", "Optional JSON data file")
	flag.Parse()

	if *templateFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --template flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Open template file
	f, err := os.Open(*templateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening template file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// Get basename of template file without extension
	base := *templateFile
	if lastSlash := strings.LastIndex(base, "/"); lastSlash >= 0 {
		base = base[lastSlash+1:]
	}
	if lastDot := strings.LastIndex(base, "."); lastDot >= 0 {
		base = base[:lastDot]
	}
	enclave := base + "-devnet"

	// Create template context
	ctx := fake.NewFakeTemplateContext(enclave)

	// Load data file if provided
	if *dataFile != "" {
		dataBytes, err := os.ReadFile(*dataFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading data file: %v\n", err)
			os.Exit(1)
		}

		var data interface{}
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing data file as JSON: %v\n", err)
			os.Exit(1)
		}

		tmpl.WithData(data)(ctx)
	}

	// Process template and write to stdout
	if err := ctx.InstantiateTemplate(f, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error processing template: %v\n", err)
		os.Exit(1)
	}
}
