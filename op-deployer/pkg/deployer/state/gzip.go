package state

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
)

type GzipData[T any] struct {
	Data *T
}

func (g *GzipData[T]) MarshalJSON() ([]byte, error) {
	jsonData, err := json.Marshal(g.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode json: %w", err)
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(jsonData); err != nil {
		return nil, fmt.Errorf("failed to write gzip data: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return json.Marshal(Base64Bytes(buf.Bytes()))
}

func (g *GzipData[T]) UnmarshalJSON(b []byte) error {
	var b64B Base64Bytes
	if err := json.Unmarshal(b, &b64B); err != nil {
		return fmt.Errorf("failed to decode gzip data: %w", err)
	}

	gr, err := gzip.NewReader(bytes.NewReader(b64B))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	var data T
	if err := json.NewDecoder(gr).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode gzip data: %w", err)
	}
	g.Data = &data
	return nil
}
