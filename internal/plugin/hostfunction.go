package plugin

import (
	"context"
	"fmt"
	"reflect"

	extism "github.com/extism/go-sdk"
	"github.com/tetratelabs/wazero/api"
)

// hostFunction, err := plugin.NewHostFunction("foo", func(ctx context.Context, param1 string, param2 int64) (string, error) {
// 	// does stuff with param1 and param2
// 	return fmt.Printf("foo called with %s and %d", param1, param2), nil
// })


func NewHostFunction[F any](name string, f F) (*HostFunction, error) {
	fValue := reflect.ValueOf(f)
	if fValue.Kind() != reflect.Func {
		return nil, fmt.Errorf("TODO")
	}

	return &HostFunction{
		Name: name,
		funcValue, fValue,
	}, nil
}

type HostFunction struct {
	Name      string
	funcValue reflect.Value
}

func (hf *HostFunction) Call(params []reflect.Value) []reflect.Value {
	return hf.funcValue.Call(params)
}

func bindExtismHostFunction(fh HostFunction) extism.HostFunction {
	//for i := 0; i < fValue.NumIn(); i++ {
	//	paramType := funcType.In(i)
	//	switch paramType.Kind() {
	//	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	//	case reflect.Float32, reflect.Float64:

	//	case reflect.Uintptr:

	//	fmt.Printf("  Parameter %d Type: %s\n", i, paramType.String())
	//}

	//convertInputs := func(stack []uint64) {

	//}

	extism.NewHostFunctionWithStack(
		fh.Name,
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {

			fType := fh.funcValue.Type()
			inputParams := make([]reflect.Value, fType.NumIn())
			for i := 0; i < fType.NumIn(); i++ {
				inputParams[i] = reflect.New(fType.In(i))
			}

			for i := 0; i < min(fType.NumIn(), len(stack); i++ {
				inputParams[i].
			}

			//if len(stack) != len(fh.funcValue.NumIn()) {

			//}

			//inputParams := convertInputs(stack)

		},
		[]extism.ValueType{ValueTypePTR},
		[]api.ValueType{ValueTypePTR},
	)

	// ValueType describes a parameter or result type mapped to a WebAssembly
	// function signature.
	//
	// The following describes how to convert between Wasm and Golang types:
	//
	//   - ValueTypeI32 - EncodeU32 DecodeU32 for uint32 / EncodeI32 DecodeI32 for int32
	//   - ValueTypeI64 - uint64(int64)
	//   - ValueTypeF32 - EncodeF32 DecodeF32 from float32
	//   - ValueTypeF64 - EncodeF64 DecodeF64 from float64
	//   - ValueTypeExternref - unintptr(unsafe.Pointer(p)) where p is any pointer
	//     type in Go (e.g. *string)

	// // ValueTypePTR represents a pointer to an Extism memory block. Alias for ValueTypeI64
	// ValueTypePTR = ValueTypeI64

	// for i := 0; i < fType.NumOut(); i++ {
	// 	returnType := fType.Out(i)
	// 	switch returnType.Kind() {
	// Float32
	// Float64
	// Complex64
	// Complex128
	// Array
	// Chan
	// Func
	// Interface
	// Map
	// Pointer
	// Slice
	// String
	// Struct
	// UnsafePointer
	//)
	//fmt.Printf("  Return %d Type: %s\n", i, returnType.String())
	//}

	// return extism.HostFunction{
	// 	Name:      "foo",
	// 	Namespace: "",
	// 	Params:    []api.ValueType{},
	// 	Returns:   []api.ValueType{},
	// }
}
