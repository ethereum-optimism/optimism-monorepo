package script

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
)

var setterFnSig = "set(bytes4,address)"
var setterFnBytes4 = bytes4(setterFnSig)

// precompileFunc is a prepared function to perform a method call / field read with ABI decoding/encoding.
type precompileFunc struct {
	goName       string
	abiSignature string
	fn           func(input []byte) ([]byte, error)
}

// bytes4 computes a 4-byte method-selector ID of a solidity method signature
func bytes4(sig string) [4]byte {
	return [4]byte(crypto.Keccak256([]byte(sig))[:4])
}

// big-endian uint64 to bytes32
func b32(v uint64) []byte {
	out := make([]byte, 32)
	binary.BigEndian.PutUint64(out[24:], v)
	return out
}

// leftPad32 to multiple of 32 bytes
func leftPad32(data []byte) []byte {
	out := bytes.Clone(data)
	if len(out)%32 == 0 {
		return out
	}
	return append(make([]byte, 32-(len(out)%32)), out...)
}

// rightPad32 to multiple of 32 bytes
func rightPad32(data []byte) []byte {
	out := bytes.Clone(data)
	if len(out)%32 == 0 {
		return out
	}
	return append(out, make([]byte, 32-(len(out)%32))...)
}

type settableField struct {
	name  string
	value *reflect.Value
}

// Precompile is a wrapper around a Go object, making it a precompile.
type Precompile[E any] struct {
	Precompile E

	fieldsOnly bool

	fieldSetter bool
	settable    map[[4]byte]*settableField

	// abiMethods is effectively the jump-table for 4-byte ABI calls to the precompile.
	abiMethods map[[4]byte]*precompileFunc
}

var _ vm.PrecompiledContract = (*Precompile[struct{}])(nil)

type PrecompileOption[E any] func(p *Precompile[E])

func WithFieldsOnly[E any](p *Precompile[E]) {
	p.fieldsOnly = true
}

func WithFieldSetter[E any](p *Precompile[E]) {
	p.fieldSetter = true
}

// NewPrecompile wraps a Go object into a Precompile.
// All exported fields and methods will have a corresponding ABI interface.
// Fields with a tag `evm:"-"` will be ignored, or can override their ABI name to x with this tag: `evm:"x"`.
// Field names and method names are adjusted to start with a lowercase character in the ABI signature.
// Method names may end with a `_X` where X must be the 4byte selector (this is sanity-checked),
// to support multiple variants of the same method with different ABI input parameters.
// Methods may return an error, which will result in a revert, rather than become an ABI encoded arg, if not nil.
// All precompile methods have 0 gas cost.
func NewPrecompile[E any](e E, opts ...PrecompileOption[E]) (*Precompile[E], error) {
	out := &Precompile[E]{
		Precompile:  e,
		abiMethods:  make(map[[4]byte]*precompileFunc),
		fieldsOnly:  false,
		fieldSetter: false,
		settable:    make(map[[4]byte]*settableField),
	}
	for _, opt := range opts {
		opt(out)
	}
	elemVal := reflect.ValueOf(e)
	// setup methods (and if pointer, the indirect methods also)
	if err := out.setupMethods(&elemVal); err != nil {
		return nil, fmt.Errorf("failed to setup methods of precompile: %w", err)
	}
	// setup fields and embedded types (if a struct)
	if err := out.setupFields(&elemVal); err != nil {
		return nil, fmt.Errorf("failed to setup fields of precompile: %w", err)
	}
	// create setter that can handle of the fields
	out.setupFieldSetter()
	return out, nil
}

// setupMethods iterates through all exposed methods of val, and sets them all up as ABI methods.
func (p *Precompile[E]) setupMethods(val *reflect.Value) error {
	if p.fieldsOnly {
		return nil
	}
	typ := val.Type()
	methodCount := val.NumMethod()
	for i := 0; i < methodCount; i++ {
		methodDef := typ.Method(i)
		if !methodDef.IsExported() {
			continue
		}
		if err := p.setupMethod(val, &methodDef); err != nil {
			return fmt.Errorf("failed to set up call-handler for method %d (%s): %w", i, methodDef.Name, err)
		}
	}
	return nil
}

// makeArgs infers a list of ABI types, from a list of Go arguments.
func makeArgs(argCount int, getType func(i int) reflect.Type) (abi.Arguments, error) {
	out := make(abi.Arguments, argCount)
	for i := 0; i < argCount; i++ {
		argType := getType(i)
		abiTyp, err := goTypeToABIType(argType)
		if err != nil {
			return nil, fmt.Errorf("failed to determine ABI type of input arg %d: %w", i, err)
		}
		out[i] = abi.Argument{
			Name: fmt.Sprintf("arg_%d", i),
			Type: abiTyp,
		}
	}
	return out, nil
}

// makeArgTypes turns a slice of ABI argument types into a slice of ABI stringified types
func makeArgTypes(args abi.Arguments) []string {
	out := make([]string, len(args))
	for i := 0; i < len(args); i++ {
		out[i] = args[i].Type.String()
	}
	return out
}

// makeArgAllocators returns a lice of Go object allocator functions, for each of the arguments.
func makeArgAllocators(argCount int, getType func(i int) reflect.Type) []func() any {
	out := make([]func() interface{}, argCount)
	for i := 0; i < argCount; i++ {
		argType := getType(i)
		out[i] = func() interface{} {
			return reflect.New(argType).Elem().Interface()
		}
	}
	return out
}

// hasTrailingError checks if the last returned argument type, if any, is a Go error.
func hasTrailingError(argCount int, getType func(i int) reflect.Type) bool {
	if argCount == 0 {
		return false
	}
	lastTyp := getType(argCount - 1)
	return lastTyp.Kind() == reflect.Interface && lastTyp.Implements(typeFor[error]())
}

// setupMethod takes a method definition, attached to selfVal,
// and builds an ABI method to handle the input decoding and output encoding around the inner Go function.
func (p *Precompile[E]) setupMethod(selfVal *reflect.Value, methodDef *reflect.Method) error {
	methodName := methodDef.Name

	abiFunctionName := methodName
	// Solidity allows multiple functions with the same name, but different input params.
	// So cut off the suffix after the last "_", to allow the different variants to be defined in Go.
	variantSuffixIndex := strings.LastIndexByte(methodName, '_')
	variantSuffix := ""
	if variantSuffixIndex >= 0 {
		abiFunctionName = methodName[:variantSuffixIndex]
		variantSuffix = methodName[variantSuffixIndex+1:] // strip out the underscore
	}
	if len(abiFunctionName) == 0 {
		return fmt.Errorf("ABI method name of %s must not be empty", methodDef.Name)
	}
	if lo := strings.ToLower(abiFunctionName[:1]); lo != abiFunctionName[:1] {
		abiFunctionName = lo + abiFunctionName[1:]
	}
	// Prepare ABI definitions of call parameters.
	inArgCount := methodDef.Type.NumIn() - 1
	if inArgCount < 0 {
		return errors.New("expected method with receiver as first argument")
	}
	getInArg := func(i int) reflect.Type {
		return methodDef.Type.In(i + 1) // +1 to account for the receiver
	}
	inArgs, err := makeArgs(inArgCount, getInArg)
	if err != nil {
		return fmt.Errorf("failed to preserve input args: %w", err)
	}
	inArgTypes := makeArgTypes(inArgs)
	methodSig := fmt.Sprintf("%v(%v)", abiFunctionName, strings.Join(inArgTypes, ","))
	byte4Sig := bytes4(methodSig)
	if variantSuffix != "" {
		if expected := fmt.Sprintf("%x", byte4Sig); expected != variantSuffix {
			return fmt.Errorf("expected variant suffix %s for ABI method %s (Go: %s), but got %s",
				expected, methodSig, methodDef.Name, variantSuffix)
		}
	}
	if m, ok := p.abiMethods[byte4Sig]; ok {
		return fmt.Errorf("method %s conflicts with existing ABI method %s (Go: %s), signature: %x",
			methodDef.Name, m.abiSignature, m.goName, byte4Sig)
	}

	outArgCount := methodDef.Type.NumOut()
	// A Go method may return an error, which we do not ABI-encode, but rather forward as revert.
	errReturn := hasTrailingError(outArgCount, methodDef.Type.Out)
	if errReturn {
		outArgCount -= 1
	}

	// Prepare ABI definitions of return parameters.
	outArgs, err := makeArgs(outArgCount, methodDef.Type.Out)
	if err != nil {
		return fmt.Errorf("failed to prepare output arg types: %w", err)
	}

	inArgAllocators := makeArgAllocators(inArgCount, getInArg)
	fn := makeFn(selfVal, &methodDef.Func, errReturn, inArgs, outArgs, inArgAllocators)

	p.abiMethods[byte4Sig] = &precompileFunc{
		goName:       methodName,
		abiSignature: methodSig,
		fn:           fn,
	}
	return nil
}

// abiToValues turns serialized ABI input data into values, which are written to the given dest slice.
// The ABI decoding happens following the given args ABI type definitions.
// Values are allocated with the given respective allocator functions.
func abiToValues(args abi.Arguments, allocators []func() any, dest []reflect.Value, input []byte) error {
	// sanity check that we have as many allocators as result destination slots
	if len(allocators) != len(dest) {
		return fmt.Errorf("have %d allocators, but %d destinations", len(allocators), len(dest))
	}
	// Unpack the ABI data into default Go types
	inVals, err := args.UnpackValues(input)
	if err != nil {
		return fmt.Errorf("failed to decode input: %x\nerr: %w", input, err)
	}
	// Sanity check that the ABI util returned the expected number of inputs
	if len(inVals) != len(allocators) {
		return fmt.Errorf("expected %d args, got %d", len(allocators), len(inVals))
	}
	for i, inAlloc := range allocators {
		argSrc := inVals[i]
		argDest := inAlloc()
		argDest, err = convertType(argSrc, argDest)
		if err != nil {
			return fmt.Errorf("failed to convert arg %d from Go type %T to %T: %w", i, argSrc, argDest, err)
		}
		dest[i] = reflect.ValueOf(argDest)
	}
	return nil
}

// makeFn is a helper function to perform a method call:
// - ABI decoding of input
// - type conversion of inputs
// - actual function Go call
// - handling of error return value
// - and ABI encoding of outputs
func makeFn(selfVal, methodVal *reflect.Value, errReturn bool, inArgs, outArgs abi.Arguments, inArgAllocators []func() any) func(input []byte) ([]byte, error) {
	return func(input []byte) ([]byte, error) {
		// Convert each default Go type into the expected opinionated Go type
		callArgs := make([]reflect.Value, 1+len(inArgAllocators))
		callArgs[0] = *selfVal
		err := abiToValues(inArgs, inArgAllocators, callArgs[1:], input)
		if err != nil {
			return nil, err
		}
		// Call the precompile Go function
		returnReflectVals := methodVal.Call(callArgs)
		// Collect the return values
		returnVals := make([]interface{}, len(returnReflectVals))
		for i := range returnReflectVals {
			returnVals[i] = returnReflectVals[i].Interface()
		}
		if errReturn {
			errIndex := len(returnVals) - 1
			if errV := returnVals[errIndex]; errV != nil {
				if err, ok := errV.(error); ok {
					return nil, err
				}
			}
			returnVals = returnVals[:errIndex]
		}
		// Encode the return values
		out, err := outArgs.PackValues(returnVals)
		if err != nil {
			return nil, fmt.Errorf("failed to encode return data: %w", err)
		}
		return out, nil
	}
}

// convertType is a helper to run the Geth type conversion util,
// forcing one Go type into another approximately equivalent Go type
// (handling pointers and underlying equivalent types).
func convertType(src, dest any) (out any, err error) {
	defer func() {
		if rErr := recover(); rErr != nil {
			err = fmt.Errorf("ConvertType fail: %v", rErr)
		}
	}()
	out = abi.ConvertType(src, dest) // no error return, just panics if invalid.
	return
}

// goTypeToABIType infers the geth ABI type definition from a Go reflect type definition.
func goTypeToABIType(typ reflect.Type) (abi.Type, error) {
	solType, internalType, err := goTypeToSolidityType(typ)
	if err != nil {
		return abi.Type{}, err
	}
	return abi.NewType(solType, internalType, nil)
}

// ABIInt256 is an alias for big.Int that is represented as int256 in ABI method signature,
// since big.Int interpretation defaults to uint256.
type ABIInt256 big.Int

var abiInt256Type = typeFor[ABIInt256]()

var abiUint256Type = typeFor[uint256.Int]()

// goTypeToSolidityType converts a Go type to the solidity ABI type definition.
// The "internalType" is a quirk of the Geth ABI utils, for nested structures.
// Unfortunately we have to convert to string, not directly to ABI type structure,
// as it is the only way to initialize Geth ABI types.
func goTypeToSolidityType(typ reflect.Type) (typeDef, internalType string, err error) {
	switch typ.Kind() {
	case reflect.Int, reflect.Uint:
		return "", "", fmt.Errorf("ints must have explicit size, type not valid: %s", typ)
	case reflect.Bool, reflect.String, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strings.ToLower(typ.Kind().String()), "", nil
	case reflect.Array:
		if typ.AssignableTo(abiUint256Type) { // uint256.Int underlying Go type is [4]uint64
			return "uint256", "", nil
		}
		if typ.Elem().Kind() == reflect.Uint8 {
			if typ.Len() == 20 && typ.Name() == "Address" {
				return "address", "", nil
			}
			if typ.Len() > 32 {
				return "", "", fmt.Errorf("byte array too large: %d", typ.Len())
			}
			return fmt.Sprintf("bytes%d", typ.Len()), "", nil
		}
		elemTyp, internalTyp, err := goTypeToSolidityType(typ.Elem())
		if err != nil {
			return "", "", fmt.Errorf("unrecognized slice-elem type: %w", err)
		}
		if internalTyp != "" {
			return "", "", fmt.Errorf("nested internal types not supported: %w", err)
		}
		return fmt.Sprintf("%s[%d]", elemTyp, typ.Len()), "", nil
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			return "bytes", "", nil
		}
		elemABITyp, internalTyp, err := goTypeToSolidityType(typ.Elem())
		if err != nil {
			return "", "", fmt.Errorf("unrecognized slice-elem type: %w", err)
		}
		if internalTyp != "" {
			return "", "", fmt.Errorf("nested internal types not supported: %w", err)
		}
		return elemABITyp + "[]", "", nil
	case reflect.Struct:
		if typ.AssignableTo(abiInt256Type) {
			return "int256", "", nil
		}
		if typ.ConvertibleTo(typeFor[big.Int]()) {
			return "uint256", "", nil
		}
		// We can parse into abi.TupleTy in the future, if necessary
		return "", "", fmt.Errorf("structs are not supported, cannot handle type %s", typ)
	case reflect.Pointer:
		elemABITyp, internalTyp, err := goTypeToSolidityType(typ.Elem())
		if err != nil {
			return "", "", fmt.Errorf("unrecognized pointer-elem type: %w", err)
		}
		return elemABITyp, internalTyp, nil
	default:
		return "", "", fmt.Errorf("unrecognized typ: %s", typ)
	}
}

// setupFields registers all exported non-ignored fields as public ABI getters.
// Fields and methods of embedded structs are registered along the way.
func (p *Precompile[E]) setupFields(val *reflect.Value) error {
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return fmt.Errorf("cannot setupFields of nil value (type: %s)", val.Type())
		}
		inner := val.Elem()
		if err := p.setupFields(&inner); err != nil {
			return fmt.Errorf("failed to setupFields of inner pointer type: %w", err)
		}
		return nil
	}
	if val.Kind() != reflect.Struct {
		return nil // ignore non-struct types
	}
	typ := val.Type()
	fieldCount := val.NumField()
	for i := 0; i < fieldCount; i++ {
		fieldTyp := typ.Field(i)
		if !fieldTyp.IsExported() {
			continue
		}
		// With the "evm" struct tag set to "-", exposed fields can explicitly be ignored,
		// and will not be translated into getter functions on the precompile or further exposed.
		if tag, ok := fieldTyp.Tag.Lookup("evm"); ok && tag == "-" {
			continue
		}
		fieldVal := val.Field(i)
		if fieldTyp.Anonymous {
			// process methods and inner fields of embedded fields
			if err := p.setupMethods(&fieldVal); err != nil {
				return fmt.Errorf("failed to setup methods of embedded field %s (type: %s): %w",
					fieldTyp.Name, fieldTyp.Type, err)
			}
			if err := p.setupFields(&fieldVal); err != nil {
				return fmt.Errorf("failed to setup fields of embedded field %s (type %s): %w",
					fieldTyp.Name, fieldTyp.Type, err)
			}
			continue
		}
		if err := p.setupStructField(&fieldTyp, &fieldVal); err != nil {
			return fmt.Errorf("failed to setup struct field %s (type %s): %w", fieldTyp.Name, fieldTyp.Type, err)
		}
	}
	return nil
}

// setupStructField registers a struct field as a public-getter ABI method.
func (p *Precompile[E]) setupStructField(fieldDef *reflect.StructField, fieldVal *reflect.Value) error {
	abiFunctionName := fieldDef.Name
	if len(abiFunctionName) == 0 {
		return fmt.Errorf("ABI name of %s must not be empty", fieldDef.Name)
	}
	if lo := strings.ToLower(abiFunctionName[:1]); lo != abiFunctionName[:1] {
		abiFunctionName = lo + abiFunctionName[1:]
	}
	// The tag can override the field name
	if v, ok := fieldDef.Tag.Lookup("evm"); ok {
		abiFunctionName = v
	}
	// The ABI signature of public fields in solidity is simply a getter function of the same name.
	// The return type is not part of the ABI signature. So we just append "()" to turn it into a function.
	methodSig := abiFunctionName + "()"
	byte4Sig := bytes4(methodSig)
	if m, ok := p.abiMethods[byte4Sig]; ok {
		return fmt.Errorf("struct field %s conflicts with existing ABI method %s (Go: %s), signature: %x",
			fieldDef.Name, m.abiSignature, m.goName, byte4Sig)
	}
	// Determine the type to ABI-encode the Go field value into
	abiTyp, err := goTypeToABIType(fieldDef.Type)
	if err != nil {
		return fmt.Errorf("failed to determine ABI type of struct field of type %s: %w", fieldDef.Type, err)
	}
	outArgs := abi.Arguments{
		{
			Name: abiFunctionName,
			Type: abiTyp,
		},
	}
	// Create the getter ABI method, that will take the field value, encode it, and return it.
	fn := func(input []byte) ([]byte, error) {
		if len(input) != 0 { // 4 byte selector is already trimmed
			return nil, fmt.Errorf("unexpected input: %x", input)
		}
		v := fieldVal.Interface()
		if abiVal, ok := v.(interface{ ToABI() []byte }); ok {
			return abiVal.ToABI(), nil
		}
		if bigInt, ok := v.(*hexutil.Big); ok { // We can change this to use convertType later, if we need more generic type handling.
			v = (*big.Int)(bigInt)
		}
		outData, err := outArgs.PackValues([]any{v})
		if err != nil {
			return nil, fmt.Errorf("method %s failed to pack return data: %w", methodSig, err)
		}
		return outData, nil
	}
	p.abiMethods[byte4Sig] = &precompileFunc{
		goName:       fieldDef.Name,
		abiSignature: methodSig,
		fn:           fn,
	}
	// register field as settable
	if p.fieldSetter && fieldDef.Type.AssignableTo(typeFor[common.Address]()) {
		p.settable[byte4Sig] = &settableField{
			name:  fieldDef.Name,
			value: fieldVal,
		}
	}
	return nil
}

func (p *Precompile[E]) setupFieldSetter() {
	if !p.fieldSetter {
		return
	}
	p.abiMethods[setterFnBytes4] = &precompileFunc{
		goName:       "__fieldSetter___",
		abiSignature: setterFnSig,
		fn: func(input []byte) ([]byte, error) {
			if len(input) != 32*2 {
				return nil, fmt.Errorf("cannot set address field to %d bytes", len(input))
			}
			if [32 - 4]byte(input[4:32]) != ([32 - 4]byte{}) {
				return nil, fmt.Errorf("unexpected selector content, input: %x", input[:])
			}
			selector := [4]byte(input[:4])
			f, ok := p.settable[selector]
			if !ok {
				return nil, fmt.Errorf("unknown address field selector 0x%x", selector)
			}
			addr := common.Address(input[32*2-20 : 32*2])
			f.value.Set(reflect.ValueOf(addr))
			return nil, nil
		},
	}
}

// RequiredGas is part of the vm.PrecompiledContract interface, and all system precompiles use 0 gas.
func (p *Precompile[E]) RequiredGas(input []byte) uint64 {
	return 0
}

// Run implements the vm.PrecompiledContract interface.
// This takes the ABI calldata, finds the applicable method by selector, and then runs that method with the data.
func (p *Precompile[E]) Run(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return encodeRevert(fmt.Errorf("expected at least 4 bytes, but got '%x'", input))
	}
	sig := [4]byte(input[:4])
	params := input[4:]
	fn, ok := p.abiMethods[sig]
	if !ok {
		return encodeRevert(fmt.Errorf("unrecognized 4 byte signature: %x", sig))
	}
	out, err := fn.fn(params)
	if err != nil {
		return encodeRevert(fmt.Errorf("failed to run %s, ABI: %q, err: %w", fn.goName, fn.abiSignature, err))
	}
	return out, nil
}

// revertSelector is the ABI signature of a default error type in solidity.
var revertSelector = crypto.Keccak256([]byte("Error(string)"))[:4]

func encodeRevert(outErr error) ([]byte, error) {
	outErrStr := []byte(outErr.Error())
	out := make([]byte, 0, 4+32*2+len(outErrStr)+32)
	out = append(out, revertSelector...)              // selector
	out = append(out, b32(0x20)...)                   // offset to string
	out = append(out, b32(uint64(len(outErrStr)))...) // length of string
	out = append(out, rightPad32(outErrStr)...)       // the error message string
	return out, vm.ErrExecutionReverted               // Geth EVM will pick this up as a revert with return-data
}

// typeFor returns the [Type] that represents the type argument T.
// Note: not available yet in Go 1.21, but part of std-lib later.
func typeFor[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
