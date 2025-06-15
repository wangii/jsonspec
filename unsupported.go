package jsonspec

import "reflect"

// An UnsupportedValueError is returned by [Marshal] when attempting
// to encode an unsupported value.
type UnsupportedTypeSpecError struct {
	Type reflect.Type
}

func (e UnsupportedTypeSpecError) Error() string {
	return "json: unsupported type: " + e.Type.Name()
}

func (ue UnsupportedTypeSpecError) encode(e *specEncodeState, opts encOpts) {
	e.error(ue)
}

func newUnsupportedTypeSpecEncoder(t reflect.Type) specEncoderFunc {
	return UnsupportedTypeSpecError{t}.encode
}
