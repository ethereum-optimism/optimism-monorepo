package serialize

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
)

// Deserializable defines functionality for a type that may be deserialized from raw bytes.
type Deserializable interface {
	// Deserialize decodes raw bytes into the type.
	Deserialize(in io.Reader) error
}

// Serializable defines functionality for a type that may be serialized to raw bytes.
type Serializable interface {
	// Serialize encodes the type as raw bytes.
	Serialize(out io.Writer) error
}

func LoadSerializedBinary[X any](inputPath string) (*X, error) {
	if inputPath == "" {
		return nil, errors.New("no path specified")
	}
	var f io.ReadCloser
	f, err := ioutil.OpenDecompressed(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", inputPath, err)
	}
	defer f.Close()

	var x X
	serializable, ok := reflect.ValueOf(&x).Interface().(Deserializable)
	if !ok {
		return nil, fmt.Errorf("%T is not a Serializable", x)
	}
	err = serializable.Deserialize(f)
	if err != nil {
		return nil, err
	}
	return &x, nil
}

func WriteSerializedBinary(value Serializable, target ioutil.OutputTarget) error {
	out, closer, abort, err := target()
	if err != nil {
		return err
	}
	if out == nil {
		return nil // Nothing to write to so skip generating content entirely
	}
	defer abort()
	err = value.Serialize(out)
	if err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}
	if err := closer.Close(); err != nil {
		return fmt.Errorf("failed to finish write: %w", err)
	}
	return nil
}
