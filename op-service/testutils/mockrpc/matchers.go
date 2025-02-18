package mockrpc

import (
	"bytes"
	"encoding/json"
	"sync"
)

type ParamsMatcher func(params json.RawMessage) bool

func AnyParamsMatcher() ParamsMatcher {
	return func(params json.RawMessage) bool {
		return true
	}
}

func NullMatcher() ParamsMatcher {
	return func(params json.RawMessage) bool {
		return isNullish(params)
	}
}

var bufPool = &sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuf() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func putBuf(b *bytes.Buffer) {
	b.Reset()
	bufPool.Put(b)
}

// JSONParamsMatcher returns a ParamsMatcher that compares the JSON representation
// of the expected and actual parameters. json.Indent is used to canonicalize the
// JSON representation. Newlines are removed from the input prior to indentation.
func JSONParamsMatcher(expected json.RawMessage) ParamsMatcher {
	if isNullish(expected) {
		return NullMatcher()
	}

	replaced := bytes.ReplaceAll(expected, []byte("\n"), nil)

	expDst := getBuf()
	if err := json.Indent(expDst, replaced, "", ""); err != nil {
		panic(err)
	}
	expStr := expDst.String()
	putBuf(expDst)

	return func(params json.RawMessage) bool {
		paramsDst := getBuf()
		defer putBuf(paramsDst)

		replaced := bytes.ReplaceAll(params, []byte("\n"), nil)
		if err := json.Indent(paramsDst, replaced, "", ""); err != nil {
			return false
		}

		actStr := paramsDst.String()
		return expStr == actStr
	}
}

func isNullish(params json.RawMessage) bool {
	return params == nil || string(params) == "null"
}
