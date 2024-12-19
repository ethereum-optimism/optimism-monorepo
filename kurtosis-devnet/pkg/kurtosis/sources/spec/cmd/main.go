package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/spec"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl/fake"
)

func main() {
	// Parse command line flags
	yamlFile := flag.String("file", "", "Path to YAML enclave definition file")
	flag.Parse()

	if *yamlFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --file flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Open and read the YAML file
	f, err := os.Open(*yamlFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening YAML file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// Get basename of template file without extension
	base := *yamlFile
	if lastSlash := strings.LastIndex(base, "/"); lastSlash >= 0 {
		base = base[lastSlash+1:]
	}
	if lastDot := strings.LastIndex(base, "."); lastDot >= 0 {
		base = base[:lastDot]
	}
	enclave := base + "-devnet"

	// Create template context and expand template
	ctx := fake.NewFakeTemplateContext(enclave)
	var buf bytes.Buffer
	if err := ctx.InstantiateTemplate(f, &buf); err != nil {
		fmt.Fprintf(os.Stderr, "Error expanding template: %v\n", err)
		os.Exit(1)
	}

	// Parse the spec from expanded template
	enclaveSpec, err := spec.NewSpec().ExtractData(&buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing spec: %v\n", err)
		os.Exit(1)
	}

	// Encode as JSON and write to stdout
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(enclaveSpec); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}
