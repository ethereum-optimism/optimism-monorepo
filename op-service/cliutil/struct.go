package cliutil

import (
	"encoding"
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

// PopulateStruct populates a struct with values from CLI context based on `cli` tags.
func PopulateStruct(cfg any, ctx *cli.Context) error {
	// Get reflected value of the config struct
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("config must be a pointer to struct")
	}
	v = v.Elem()
	t := v.Type()

	// Iterate over struct fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Get CLI tag
		cliTag := field.Tag.Get("cli")
		if cliTag == "" {
			continue
		}

		// Skip if field is not settable
		if !fieldValue.CanSet() {
			continue
		}

		// Handle different types
		if err := setFieldValue(fieldValue, field.Type, ctx, cliTag); err != nil {
			return fmt.Errorf("failed to set field %s: %w", field.Name, err)
		}
	}

	return nil
}

// setFieldValue sets the appropriate value based on the field type
func setFieldValue(fieldValue reflect.Value, fieldType reflect.Type, ctx *cli.Context, flag string) error {
	switch fieldType.Kind() {
	case reflect.String:
		fieldValue.SetString(ctx.String(flag))
	case reflect.Bool:
		fieldValue.SetBool(ctx.Bool(flag))
	case reflect.Int, reflect.Int64:
		fieldValue.SetInt(int64(ctx.Int(flag)))
	case reflect.Uint64:
		fieldValue.SetUint(ctx.Uint64(flag))
	case reflect.Ptr:
		if !ctx.IsSet(flag) {
			return nil // Skip if flag is not set
		}
		// Handle pointer types
		return handlePointerType(fieldValue, fieldType, ctx, flag)
	default:
		// Handle special types
		return handleSpecialTypes(fieldValue, fieldType, ctx, flag)
	}
	return nil
}

// handlePointerType handles pointer type fields
func handlePointerType(fieldValue reflect.Value, fieldType reflect.Type, ctx *cli.Context, flag string) error {
	// Create a new instance of the pointed-to type
	elem := reflect.New(fieldType.Elem())

	// If it implements TextUnmarshaler, use that
	if unmarshaler, ok := elem.Interface().(encoding.TextUnmarshaler); ok {
		if err := unmarshaler.UnmarshalText([]byte(ctx.String(flag))); err != nil {
			return err
		}
		fieldValue.Set(elem)
		return nil
	}

	// Handle other pointer types as needed
	return fmt.Errorf("unsupported pointer type: %v", fieldType)
}

// handleSpecialTypes handles non-primitive types that need special handling
func handleSpecialTypes(fieldValue reflect.Value, fieldType reflect.Type, ctx *cli.Context, flag string) error {
	// Handle common.Address
	if fieldType == reflect.TypeOf(common.Address{}) {
		if !ctx.IsSet(flag) {
			return nil
		}
		addrStr := ctx.String(flag)
		if !common.IsHexAddress(addrStr) {
			return fmt.Errorf("invalid address: %s", addrStr)
		}
		addr := common.HexToAddress(addrStr)
		fieldValue.Set(reflect.ValueOf(addr))
		return nil
	}

	// If type implements TextUnmarshaler
	if unmarshaler, ok := fieldValue.Interface().(encoding.TextUnmarshaler); ok {
		return unmarshaler.UnmarshalText([]byte(ctx.String(flag)))
	}

	return fmt.Errorf("unsupported type: %v", fieldType)
}
