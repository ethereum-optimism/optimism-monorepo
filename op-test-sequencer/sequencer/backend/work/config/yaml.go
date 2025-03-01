package config

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
)

// YamlLoader is a Loader that loads a builders configuration from a YAML file path.
type YamlLoader struct {
	Path string
}

var _ work.Loader = (*YamlLoader)(nil)

func (l *YamlLoader) Load(ctx context.Context) (work.Starter, error) {
	data, err := os.ReadFile(l.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var out Ensemble
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // ensure all the fields are known. Config correctness is critical.
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}
	return &out, nil
}
