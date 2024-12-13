package jsonutil

import "encoding/json"

// MergeJSON merges the provided overrides into the input struct. Fields
// must be JSON-serializable for this to work. Overrides are applied in
// order of precedence - i.e., the last overrides will override keys from
// all preceding overrides.
func MergeJSON[T any](in T, overrides ...map[string]any) (T, error) {
	var out T
	inJSON, err := json.Marshal(in)
	if err != nil {
		return out, err
	}

	var tmpMap map[string]interface{}
	if err := json.Unmarshal(inJSON, &tmpMap); err != nil {
		return out, err
	}

	for _, override := range overrides {
		for k, v := range override {
			tmpMap[k] = v
		}
	}

	inJSON, err = json.Marshal(tmpMap)
	if err != nil {
		return out, err
	}

	if err := json.Unmarshal(inJSON, &out); err != nil {
		return out, err
	}

	return out, nil
}
