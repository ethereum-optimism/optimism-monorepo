package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fatih/color"
)

// ImmutableReference represents an immutable reference in the contract bytecode
type ImmutableReference struct {
	Offset int
	Length int
	Value  string
}

// BytecodeDifference represents a difference between expected and actual bytecode
type BytecodeDifference struct {
	Start         int
	Length        int
	Expected      string
	Actual        string
	InImmutable   bool
	ImmutableName string
}

// currentDiff is a helper struct for tracking differences during comparison
type currentDiff struct {
	Start         int
	Expected      []string
	Actual        []string
	InImmutable   bool
	ImmutableName string
}

func main() {
	// Parse command line arguments
	address := flag.String("address", "", "Contract address to check")
	artifactPath := flag.String("artifact", "", "Path to the contract artifact JSON file")
	rpcURL := flag.String("rpc", "", "RPC URL for the network")
	flag.Parse()

	if *rpcURL == "" {
		color.Red("Error: RPC URL is required")
		flag.Usage()
		os.Exit(1)
	}

	color.Cyan("Comparing contract at %s with artifact %s", *address, *artifactPath)

	// Load the artifact
	artifact, err := loadArtifact(*artifactPath)
	if err != nil {
		color.Red("Error loading artifact: %v", err)
		os.Exit(1)
	}

	// Get expected bytecode from artifact
	expectedBytecode, err := getDeployedBytecode(artifact)
	if err != nil {
		color.Red("Error: %v", err)
		os.Exit(1)
	}

	// Get immutable references
	immutableRefs, err := getImmutableReferences(artifact)
	if err != nil {
		color.Red("Error: %v", err)
		os.Exit(1)
	}

	// Get actual bytecode from the network
	actualBytecode, err := getOnchainBytecode(*address, *rpcURL)
	if err != nil {
		color.Red("Error: %v", err)
		os.Exit(1)
	}

	// Find differences
	differences, err := findDifferences(expectedBytecode, actualBytecode, immutableRefs)
	if err != nil {
		color.Red("Error: %v", err)
		os.Exit(1)
	}

	// Print results
	printDifferences(differences, immutableRefs)

	// Exit with error code if there are non-immutable differences
	for _, diff := range differences {
		if !diff.InImmutable {
			os.Exit(1)
		}
	}

	color.Green("✓ Contract bytecode matches the artifact (accounting for immutable references).")
}

func loadArtifact(path string) (map[string]any, error) {
	if path == "" {
		return nil, fmt.Errorf("artifact path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact file: %w", err)
	}

	var artifact map[string]any
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return artifact, nil
}

func getDeployedBytecode(artifact map[string]any) (string, error) {
	// Check for Forge/Foundry artifact format
	if deployedBytecode, ok := artifact["deployedBytecode"].(map[string]any); ok {
		if object, ok := deployedBytecode["object"].(string); ok {
			return object, nil
		}
	}

	// Check for standard artifact formats
	if deployedBytecode, ok := artifact["deployedBytecode"].(string); ok {
		return deployedBytecode, nil
	}

	// Check for bytecode field
	if bytecode, ok := artifact["bytecode"].(map[string]any); ok {
		if object, ok := bytecode["object"].(string); ok {
			return object, nil
		}
	} else if bytecode, ok := artifact["bytecode"].(string); ok {
		return bytecode, nil
	}

	return "", fmt.Errorf("could not find deployedBytecode in artifact")
}

func getVariableNameFromAST(artifact map[string]any, varID string) string {
	// Remove any prefix from the ID (sometimes IDs are prefixed with a path)
	cleanID := varID
	if strings.Contains(varID, ":") {
		parts := strings.Split(varID, ":")
		cleanID = parts[len(parts)-1]
	}

	// Try to convert to int
	idInt, err := strconv.Atoi(cleanID)
	if err != nil {
		return varID
	}

	// Try to find the AST node
	if ast, ok := artifact["ast"].(map[string]any); ok {
		// Recursively search for the node with matching ID
		name := findNodeName(ast, idInt)
		if name != "" {
			return name
		}
	}

	// Fallback to using the ID if we can't find the name
	return varID
}

func findNodeName(node any, targetID int) string {
	switch n := node.(type) {
	case map[string]any:
		// Check if this is the node we're looking for
		if id, ok := n["id"].(float64); ok && int(id) == targetID {
			if name, ok := n["name"].(string); ok {
				return name
			}
		}

		// Recursively search in all child nodes
		for _, value := range n {
			result := findNodeName(value, targetID)
			if result != "" {
				return result
			}
		}
	case []any:
		// Search in list items
		for _, item := range n {
			result := findNodeName(item, targetID)
			if result != "" {
				return result
			}
		}
	}
	return ""
}

func getImmutableReferences(artifact map[string]any) (map[string][]ImmutableReference, error) {
	references := make(map[string][]ImmutableReference)

	var immutableRefs map[string]any

	// Handle Forge/Foundry artifact format
	if deployedBytecode, ok := artifact["deployedBytecode"].(map[string]any); ok {
		if refs, ok := deployedBytecode["immutableReferences"].(map[string]any); ok {
			immutableRefs = refs
		} else {
			return references, nil // No immutable references found
		}
	} else if refs, ok := artifact["immutableReferences"].(map[string]any); ok {
		// Handle standard artifact format
		immutableRefs = refs
	} else {
		return references, nil // No immutable references found
	}

	// Process the references
	for varID, refs := range immutableRefs {
		// Get the variable name from AST
		varName := getVariableNameFromAST(artifact, varID)
		references[varName] = []ImmutableReference{}

		refsList, ok := refs.([]any)
		if !ok {
			continue
		}

		for _, ref := range refsList {
			var start, length int

			// Handle different formats of immutable references
			if refMap, ok := ref.(map[string]any); ok {
				if startVal, ok := refMap["start"].(float64); ok {
					start = int(startVal)
				}
				if lengthVal, ok := refMap["length"].(float64); ok {
					length = int(lengthVal)
				}
			} else if refArray, ok := ref.([]any); ok && len(refArray) >= 2 {
				// Some formats use [start, length] array
				if startVal, ok := refArray[0].(float64); ok {
					start = int(startVal)
				}
				if lengthVal, ok := refArray[1].(float64); ok {
					length = int(lengthVal)
				}
			} else {
				color.Yellow("Warning: Unrecognized immutable reference format: %v", ref)
				continue
			}

			references[varName] = append(references[varName], ImmutableReference{
				Offset: start,
				Length: length,
				Value:  "",
			})
		}
	}

	return references, nil
}

func getOnchainBytecode(address string, rpcURL string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("contract address is required")
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return "", fmt.Errorf("failed to connect to RPC at %s: %w", rpcURL, err)
	}

	code, err := client.CodeAt(context.Background(), common.HexToAddress(address), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get code at address %s: %w", address, err)
	}

	if len(code) == 0 {
		return "", fmt.Errorf("no code found at address %s", address)
	}

	return "0x" + hex.EncodeToString(code), nil
}

func isInImmutableReference(
	position int,
	immutableRefs map[string][]ImmutableReference,
) (bool, string, *ImmutableReference) {
	for varName, refs := range immutableRefs {
		for i := range refs {
			ref := &refs[i]
			if ref.Offset <= position && position < ref.Offset+ref.Length {
				return true, varName, ref
			}
		}
	}
	return false, "", nil
}

func findDifferences(
	expectedBytecode string,
	actualBytecode string,
	immutableRefs map[string][]ImmutableReference,
) ([]BytecodeDifference, error) {
	// Remove '0x' prefix if present
	expected := strings.TrimPrefix(expectedBytecode, "0x")
	actual := strings.TrimPrefix(actualBytecode, "0x")

	// Convert to bytes for comparison
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return nil, fmt.Errorf("failed to decode expected bytecode: %w", err)
	}

	actualBytes, err := hex.DecodeString(actual)
	if err != nil {
		return nil, fmt.Errorf("failed to decode actual bytecode: %w", err)
	}

	// Check length differences
	if len(expectedBytes) != len(actualBytes) {
		color.Yellow("Warning: Bytecode length mismatch. Expected: %d, Actual: %d",
			len(expectedBytes), len(actualBytes))
	}

	// Use the shorter length for comparison
	compareLength := min(len(expectedBytes), len(actualBytes))

	// Initialize all immutable reference values
	for _, refs := range immutableRefs {
		for i := range refs {
			refs[i].Value = ""
		}
	}

	differences := []BytecodeDifference{}
	var currDiff *currentDiff = nil

	for i := 0; i < compareLength; i++ {
		inImmutable, varName, ref := isInImmutableReference(i, immutableRefs)

		// If we're in an immutable reference, collect the value
		if inImmutable && ref != nil {
			// Add this byte to the immutable value
			ref.Value += fmt.Sprintf("%02x", actualBytes[i])

			// If bytes differ and we're in an immutable reference, that's expected
			if expectedBytes[i] != actualBytes[i] {
				if currDiff == nil {
					currDiff = &currentDiff{
						Start:         i,
						Expected:      []string{},
						Actual:        []string{},
						InImmutable:   true,
						ImmutableName: varName,
					}
				} else if !currDiff.InImmutable {
					// We were tracking a non-immutable diff, finish it and start a new one
					differences = append(differences, BytecodeDifference{
						Start:         currDiff.Start,
						Length:        len(currDiff.Expected),
						Expected:      strings.Join(currDiff.Expected, ""),
						Actual:        strings.Join(currDiff.Actual, ""),
						InImmutable:   currDiff.InImmutable,
						ImmutableName: currDiff.ImmutableName,
					})
					currDiff = &currentDiff{
						Start:         i,
						Expected:      []string{},
						Actual:        []string{},
						InImmutable:   true,
						ImmutableName: varName,
					}
				}

				currDiff.Expected = append(currDiff.Expected, fmt.Sprintf("%02x", expectedBytes[i]))
				currDiff.Actual = append(currDiff.Actual, fmt.Sprintf("%02x", actualBytes[i]))
			} else if currDiff != nil && currDiff.InImmutable {
				// End of a difference section within an immutable reference
				differences = append(differences, BytecodeDifference{
					Start:         currDiff.Start,
					Length:        len(currDiff.Expected),
					Expected:      strings.Join(currDiff.Expected, ""),
					Actual:        strings.Join(currDiff.Actual, ""),
					InImmutable:   currDiff.InImmutable,
					ImmutableName: currDiff.ImmutableName,
				})
				currDiff = nil
			}
		} else {
			// Not in an immutable reference - any difference is an error
			if expectedBytes[i] != actualBytes[i] {
				if currDiff == nil {
					currDiff = &currentDiff{
						Start:         i,
						Expected:      []string{},
						Actual:        []string{},
						InImmutable:   false,
						ImmutableName: "",
					}
				} else if currDiff.InImmutable {
					// We were tracking an immutable diff, finish it and start a new one
					differences = append(differences, BytecodeDifference{
						Start:         currDiff.Start,
						Length:        len(currDiff.Expected),
						Expected:      strings.Join(currDiff.Expected, ""),
						Actual:        strings.Join(currDiff.Actual, ""),
						InImmutable:   currDiff.InImmutable,
						ImmutableName: currDiff.ImmutableName,
					})
					currDiff = &currentDiff{
						Start:         i,
						Expected:      []string{},
						Actual:        []string{},
						InImmutable:   false,
						ImmutableName: "",
					}
				}

				currDiff.Expected = append(currDiff.Expected, fmt.Sprintf("%02x", expectedBytes[i]))
				currDiff.Actual = append(currDiff.Actual, fmt.Sprintf("%02x", actualBytes[i]))
			} else if currDiff != nil && !currDiff.InImmutable {
				// End of a difference section outside immutable reference
				differences = append(differences, BytecodeDifference{
					Start:         currDiff.Start,
					Length:        len(currDiff.Expected),
					Expected:      strings.Join(currDiff.Expected, ""),
					Actual:        strings.Join(currDiff.Actual, ""),
					InImmutable:   currDiff.InImmutable,
					ImmutableName: currDiff.ImmutableName,
				})
				currDiff = nil
			}
		}
	}

	// Don't forget the last difference if we reached the end
	if currDiff != nil {
		differences = append(differences, BytecodeDifference{
			Start:         currDiff.Start,
			Length:        len(currDiff.Expected),
			Expected:      strings.Join(currDiff.Expected, ""),
			Actual:        strings.Join(currDiff.Actual, ""),
			InImmutable:   currDiff.InImmutable,
			ImmutableName: currDiff.ImmutableName,
		})
	}

	return differences, nil
}

func printDifferences(
	differences []BytecodeDifference,
	immutableRefs map[string][]ImmutableReference,
) {
	// Separate immutable and non-immutable differences
	var nonImmutableDiffs []BytecodeDifference
	var immutableDiffs []BytecodeDifference

	for _, diff := range differences {
		if diff.InImmutable {
			immutableDiffs = append(immutableDiffs, diff)
		} else {
			nonImmutableDiffs = append(nonImmutableDiffs, diff)
		}
	}

	// Print summary
	color.Cyan("\n=== Bytecode Comparison Summary ===")
	fmt.Printf("Total differences: %d\n", len(differences))
	fmt.Printf("  - In immutable references: %d\n", len(immutableDiffs))
	fmt.Printf("  - In code: %d\n", len(nonImmutableDiffs))

	// Print non-immutable differences (these are errors)
	if len(nonImmutableDiffs) > 0 {
		color.Red("\n=== Unexpected Differences in Code ===")
		for _, diff := range nonImmutableDiffs {
			color.Red("Position %d-%d:", diff.Start, diff.Start+diff.Length-1)
			fmt.Printf("  Expected: 0x%s\n", diff.Expected)
			fmt.Printf("  Actual:   0x%s\n", diff.Actual)
		}
		color.Red("\n⚠️  The contract bytecode does not match the artifact!")
	} else {
		color.Green("\n✓ No unexpected differences in code.")
	}

	// Print immutable references
	color.Cyan("\n=== Immutable References ===")
	if len(immutableRefs) == 0 {
		fmt.Println("No immutable references found in the artifact.")
	} else {
		for varName, refs := range immutableRefs {
			color.Yellow("\n%s:", varName)
			for i, ref := range refs {
				if ref.Value != "" {
					fmt.Printf("  [%d] Offset: %d, Length: %d\n", i, ref.Offset, ref.Length)
					fmt.Printf("      Value: 0x%s\n", ref.Value)
				} else {
					fmt.Printf("  [%d] Offset: %d, Length: %d\n", i, ref.Offset, ref.Length)
					fmt.Printf("      Value: (not modified)\n")
				}
			}
		}
	}
}
