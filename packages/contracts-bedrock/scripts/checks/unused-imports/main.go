package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

var importPattern = regexp.MustCompile(`import\s*{([^}]+)}`)
var asPattern = regexp.MustCompile(`(\S+)\s+as\s+(\S+)`)

func main() {
	if err := run(); err != nil {
		writeStderr("an error occurred: %v", err)
		os.Exit(1)
	}
}

func writeStderr(msg string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	var hasErr atomic.Bool
	var outMtx sync.Mutex
	fail := func(msg string, args ...any) {
		outMtx.Lock()
		writeStderr("❌  "+msg, args...)
		outMtx.Unlock()
		hasErr.Store(true)
	}

	dirs := []string{"src", "scripts", "test"}
	sem := make(chan struct{}, runtime.NumCPU())

	for _, dir := range dirs {
		dirPath := filepath.Join(cwd, dir)
		if _, err := os.Stat(dirPath); errors.Is(err, os.ErrNotExist) {
			continue
		}

		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".sol") {
				sem <- struct{}{}
				go func() {
					defer func() { <-sem }()
					processFile(path, fail)
				}()
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", dir, err)
		}
	}

	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}

	if hasErr.Load() {
		return errors.New("unused imports check failed, see logs above")
	}

	return nil
}

func processFile(filePath string, fail func(string, ...any)) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fail("%s: failed to read file: %v", filePath, err)
		return
	}

	imports := findImports(string(content))
	unusedImports := checkUnusedImports(imports, string(content))

	if len(unusedImports) > 0 {
		fail("File: %s\nUnused imports:", filePath)
		for _, unused := range unusedImports {
			fail("  - %s", unused)
		}
	}
}

func findImports(content string) []string {
	var imports []string
	matches := importPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			importList := strings.Split(match[1], ",")
			for _, imp := range importList {
				imports = append(imports, strings.TrimSpace(imp))
			}
		}
	}
	return imports
}

func checkUnusedImports(imports []string, content string) []string {
	var unusedImports []string
	for _, imp := range imports {
		searchTerm := imp
		displayName := imp

		if match := asPattern.FindStringSubmatch(imp); len(match) > 2 {
			searchTerm = match[2]
			displayName = fmt.Sprintf("%s as %s", match[1], match[2])
		}

		if !isImportUsed(searchTerm, content) {
			unusedImports = append(unusedImports, displayName)
		}
	}
	return unusedImports
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
