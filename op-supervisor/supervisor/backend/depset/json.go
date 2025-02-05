package depset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// JsonDependencySetLoader loads a dependency set from a file-path.
type JsonDependencySetLoader struct {
	Path string
}

func (j *JsonDependencySetLoader) LoadDependencySet(ctx context.Context) (DependencySet, error) {
	f, err := os.Open(j.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open dependency set: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var out StaticConfigDependencySet
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode dependency set: %w", err)
	}
	return &out, nil
}

var _ DependencySetSource = (*JsonDependencySetLoader)(nil)
