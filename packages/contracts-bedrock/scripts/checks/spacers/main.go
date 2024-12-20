package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/scripts/checks/common"
)

func parseVariableLength(variableType string, types map[string]solc.StorageLayoutType) (int, error) {
	if t, exists := types[variableType]; exists {
		return int(t.NumberOfBytes), nil
	}

	if strings.HasPrefix(variableType, "t_mapping") {
		return 32, nil
	} else if strings.HasPrefix(variableType, "t_uint") {
		re := regexp.MustCompile(`uint(\d+)`)
		matches := re.FindStringSubmatch(variableType)
		if len(matches) > 1 {
			bitSize, _ := strconv.Atoi(matches[1])
			return bitSize / 8, nil
		}
	} else if strings.HasPrefix(variableType, "t_bytes_") {
		return 32, nil
	} else if strings.HasPrefix(variableType, "t_bytes") {
		re := regexp.MustCompile(`bytes(\d+)`)
		matches := re.FindStringSubmatch(variableType)
		if len(matches) > 1 {
			return strconv.Atoi(matches[1])
		}
	} else if strings.HasPrefix(variableType, "t_address") {
		return 20, nil
	} else if strings.HasPrefix(variableType, "t_bool") {
		return 1, nil
	} else if strings.HasPrefix(variableType, "t_array") {
		re := regexp.MustCompile(`^t_array\((\w+)\)(\d+)`)
		matches := re.FindStringSubmatch(variableType)
		if len(matches) > 2 {
			innerType := matches[1]
			size, _ := strconv.Atoi(matches[2])
			length, err := parseVariableLength(innerType, types)
			if err != nil {
				return 0, err
			}
			return length * size, nil
		}
	}

	return 0, fmt.Errorf("unsupported type %s, add it to the script", variableType)
}

func validateSpacer(variable solc.StorageLayoutEntry, types map[string]solc.StorageLayoutType) []error {
	var errors []error

	parts := strings.Split(variable.Label, "_")
	if len(parts) != 4 {
		return []error{fmt.Errorf("invalid spacer name format: %s", variable.Label)}
	}

	expectedSlot, _ := strconv.Atoi(parts[1])
	expectedOffset, _ := strconv.Atoi(parts[2])
	expectedLength, _ := strconv.Atoi(parts[3])

	actualLength, err := parseVariableLength(variable.Type, types)
	if err != nil {
		return []error{err}
	}

	if int(variable.Slot) != expectedSlot {
		errors = append(errors, fmt.Errorf("%s %s is in slot %d but should be in %d",
			variable.Contract, variable.Label, variable.Slot, expectedSlot))
	}

	if int(variable.Offset) != expectedOffset {
		errors = append(errors, fmt.Errorf("%s %s is at offset %d but should be at %d",
			variable.Contract, variable.Label, variable.Offset, expectedOffset))
	}

	if actualLength != expectedLength {
		errors = append(errors, fmt.Errorf("%s %s is %d bytes long but should be %d",
			variable.Contract, variable.Label, actualLength, expectedLength))
	}

	return errors
}

func processFile(path string) (*common.Void, []error) {
	artifact, err := common.ReadForgeArtifact(path)
	if err != nil {
		return nil, []error{err}
	}

	if artifact.StorageLayout == nil {
		return nil, nil
	}

	var errors []error
	for _, variable := range artifact.StorageLayout.Storage {
		if strings.HasPrefix(variable.Label, "spacer_") {
			if errs := validateSpacer(variable, artifact.StorageLayout.Types); len(errs) > 0 {
				errors = append(errors, errs...)
				continue
			}
		}
	}

	return nil, errors
}

func main() {
	if _, err := common.ProcessFilesGlob(
		[]string{"forge-artifacts/**/*.json"},
		[]string{"forge-artifacts/**/CrossDomainMessengerLegacySpacer{0,1}.json"},
		processFile,
	); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
