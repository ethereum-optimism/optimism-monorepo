package jsonutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/BurntSushi/toml"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
)

type Decoder interface {
	Decode(v interface{}) error
}

type DecoderFactory func(r io.Reader) Decoder

type Encoder interface {
	Encode(v interface{}) error
}

type EncoderFactory func(w io.Writer) Encoder

type jsonDecoder struct {
	d *json.Decoder
}

func newJSONDecoder(r io.Reader) Decoder {
	return &jsonDecoder{
		d: json.NewDecoder(r),
	}
}

func (d *jsonDecoder) Decode(v interface{}) error {
	if err := d.d.Decode(v); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}
	if _, err := d.d.Token(); err != io.EOF {
		return errors.New("unexpected trailing data")
	}
	return nil
}

type tomlDecoder struct {
	r io.Reader
}

func newTOMLDecoder(r io.Reader) Decoder {
	return &tomlDecoder{
		r: r,
	}
}

func (d *tomlDecoder) Decode(v interface{}) error {
	if _, err := toml.NewDecoder(d.r).Decode(v); err != nil {
		return fmt.Errorf("failed to decode TOML: %w", err)
	}
	return nil
}

type jsonEncoder struct {
	e *json.Encoder
}

func newJSONEncoder(w io.Writer) Encoder {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return &jsonEncoder{
		e: e,
	}
}

func (e *jsonEncoder) Encode(v interface{}) error {
	if err := e.e.Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

type tomlEncoder struct {
	w io.Writer
}

func newTOMLEncoder(w io.Writer) Encoder {
	return &tomlEncoder{
		w: w,
	}
}

func (e *tomlEncoder) Encode(v interface{}) error {
	if err := toml.NewEncoder(e.w).Encode(v); err != nil {
		return fmt.Errorf("failed to encode TOML: %w", err)
	}
	return nil
}

func LoadJSON[X any](inputPath string) (*X, error) {
	return load[X](inputPath, newJSONDecoder)
}

func LoadTOML[X any](inputPath string) (*X, error) {
	return load[X](inputPath, newTOMLDecoder)
}

func load[X any](inputPath string, dec DecoderFactory) (*X, error) {
	if inputPath == "" {
		return nil, errors.New("no path specified")
	}
	var f io.ReadCloser
	f, err := ioutil.OpenDecompressed(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", inputPath, err)
	}
	defer f.Close()
	var state X
	if err := dec(f).Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode file %q: %w", inputPath, err)
	}
	return &state, nil
}

func WriteJSON[X any](value X, target ioutil.OutputTarget) error {
	return write(value, target, newJSONEncoder)
}

func WriteTOML[X any](value X, target ioutil.OutputTarget) error {
	return write(value, target, newTOMLEncoder)
}

func write[X any](value X, target ioutil.OutputTarget, enc EncoderFactory) error {
	out, closer, abort, err := target()
	if err != nil {
		return err
	}
	if out == nil {
		return nil // No output stream selected so skip generating the content entirely
	}
	defer abort()
	if err := enc(out).Encode(value); err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}
	_, err = out.Write([]byte{'\n'})
	if err != nil {
		return fmt.Errorf("failed to append new-line: %w", err)
	}
	if err := closer.Close(); err != nil {
		return fmt.Errorf("failed to finish write: %w", err)
	}
	return nil
}
