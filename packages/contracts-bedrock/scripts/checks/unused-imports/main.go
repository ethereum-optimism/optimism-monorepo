package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

var importPattern = regexp.MustCompile(`import\s*{([^}]+)}`)
var asPattern = regexp.MustCompile(`(\S+)\s+as\s+(\S+)`)

func main() {
	if err := common.ProcessFilesGlob(
		[]string{"src/**/*.sol", "scripts/**/*.sol", "test/**/*.sol"},
		[]string{},
		processFile,
	); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func processFile(filePath string) []error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return []error{fmt.Errorf("%s: failed to read file: %w", filePath, err)}
	}

	imports := findImports(string(content))
	var unusedImports []string
	for _, imp := range imports {
		if !isImportUsed(imp, string(content)) {
			unusedImports = append(unusedImports, imp)
		}
	}

	if len(unusedImports) > 0 {
		var errors []error
		for _, unused := range unusedImports {
			errors = append(errors, fmt.Errorf("%s", unused))
		}
		return errors
	}

	return nil
}

func findImports(content string) []string {
	var imports []string
	matches := importPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			importList := strings.Split(match[1], ",")
			for _, imp := range importList {
				imp = strings.TrimSpace(imp)
				if asMatch := asPattern.FindStringSubmatch(imp); len(asMatch) > 2 {
					// Use the renamed identifier (after 'as')
					imports = append(imports, strings.TrimSpace(asMatch[2]))
				} else {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

func isImportUsed(imp, content string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		if strings.Contains(line, "import") {
			continue
		}
		if strings.Contains(line, imp) {
			return true
		}
	}
	return false
}
