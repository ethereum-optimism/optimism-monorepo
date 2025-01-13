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
	if _, err := common.ProcessFilesGlob(
		[]string{"src/**/*.sol", "scripts/**/*.sol", "test/**/*.sol", "interfaces/**/*.sol"},
		[]string{"src/dispute/lib/Types.sol"},
		processFile,
	); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func processFile(filePath string) (*common.Void, []error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, []error{fmt.Errorf("%s: failed to read file: %w", filePath, err)}
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
		return nil, errors
	}

	return nil, nil
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
	// Use a regular expression to match the import as a whole word
	wordPattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(imp))
	scanner := bufio.NewScanner(strings.NewReader(content))

	importOpen := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "//") || strings.HasPrefix(strings.TrimSpace(line), "/*") || strings.HasPrefix(strings.TrimSpace(line), "*") || strings.HasPrefix(strings.TrimSpace(line), "*/") {
			continue
		}
		if importOpen {
			if strings.Contains(line, "}") {
				importOpen = false
			}
			continue
		}
		if strings.Contains(line, "import {") {
			if !strings.Contains(line, "}") {
				importOpen = true
			}
			continue
		}

		if matched, _ := regexp.MatchString(wordPattern, line); matched {
			return true
		}
	}
	return false
}
