package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type Check func(parts []string) bool

type CheckInfo struct {
	check Check
	error string
}

var excludes = map[string]bool{}

var checks = []CheckInfo{
	{
		error: "test name parts should be in camelCase",
		check: func(parts []string) bool {
			for _, part := range parts {
				if len(part) > 0 && unicode.IsUpper(rune(part[0])) {
					return false
				}
			}
			return true
		},
	},
	{
		error: "test names should have either 3 or 4 parts, each separated by underscores",
		check: func(parts []string) bool {
			return len(parts) == 3 || len(parts) == 4
		},
	},
	{
		error: "test names should begin with \"test\", \"testFuzz\", or \"testDiff\"",
		check: func(parts []string) bool {
			return parts[0] == "test" || parts[0] == "testFuzz" || parts[0] == "testDiff"
		},
	},
	{
		error: "test names should end with either \"succeeds\", \"reverts\", \"fails\", \"works\" or \"benchmark[_num]\"",
		check: func(parts []string) bool {
			last := parts[len(parts)-1]
			if last == "succeeds" || last == "reverts" || last == "fails" || last == "works" {
				return true
			}
			if len(parts) >= 2 && parts[len(parts)-2] == "benchmark" {
				_, err := strconv.Atoi(last)
				return err == nil
			}
			return last == "benchmark"
		},
	},
	{
		error: "failure tests should have 4 parts, third part should indicate the reason for failure",
		check: func(parts []string) bool {
			last := parts[len(parts)-1]
			return len(parts) == 4 || (last != "reverts" && last != "fails")
		},
	},
}

func main() {
	cmd := exec.Command("forge", "config", "--json")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error executing forge config: %v\n", err)
		os.Exit(1)
	}

	var config map[string]interface{}
	err = json.Unmarshal(output, &config)
	if err != nil {
		fmt.Printf("Error parsing forge config: %v\n", err)
		os.Exit(1)
	}

	outDir, ok := config["out"].(string)
	if !ok {
		outDir = "out"
	}

	fmt.Println("Success:")
	var errors []string

	err = filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if excludes[strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))] {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var artifact map[string]interface{}
		err = json.Unmarshal(data, &artifact)
		if err != nil {
			return nil // Skip files that are not valid JSON
		}

		abi, ok := artifact["abi"].([]interface{})
		if !ok {
			return nil
		}

		isTest := false
		for _, element := range abi {
			if elem, ok := element.(map[string]interface{}); ok {
				if elem["name"] == "IS_TEST" {
					isTest = true
					break
				}
			}
		}

		if isTest {
			success := true
			for _, element := range abi {
				if elem, ok := element.(map[string]interface{}); ok {
					if elem["type"] == "function" {
						name, ok := elem["name"].(string)
						if !ok || !strings.HasPrefix(name, "test") {
							continue
						}

						parts := strings.Split(name, "_")
						for _, check := range checks {
							if !check.check(parts) {
								errors = append(errors, fmt.Sprintf("%s#%s: %s", path, name, check.error))
								success = false
							}
						}
					}
				}
			}

			if success {
				fmt.Printf(" - %s\n", filepath.Base(path[:len(path)-len(filepath.Ext(path))]))
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path %q: %v\n", outDir, err)
		os.Exit(1)
	}

	if len(errors) > 0 {
		fmt.Println(strings.Join(errors, "\n"))
		os.Exit(1)
	}
}
