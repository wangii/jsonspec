// Copyright 2025 Linan Wang. All rights reserved.

package jsonspec

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"
	// _ "unsafe" // for linkname
)

type TPromptParam[T any] struct {
	In      T
	OutSpec string
}

func AppendSpec[T, P any](param P) (*TPromptParam[P], error) {

	bsspec, err := SpecMarshal(reflect.TypeOf((*T)(nil)).Elem(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成模型OutSpec参数失败: %v", err)
	}

	spec := "```json\n" + string(bsspec) + "\n```"
	ret := &TPromptParam[P]{
		In:      param,
		OutSpec: spec,
	}
	return ret, nil
}

type encOpts struct {
	escapeHTML bool
	quoted     bool

	prefix string
	indent string
}

func SpecMarshal(t reflect.Type, prefix, indent string) ([]byte, error) {
	e := newSpecEncodeState()
	defer func() {
		e.ptrSeen = make(map[any]struct{})
		specEncodeStatePool.Put(e)
	}()

	err := e.marshal(t, encOpts{escapeHTML: true, prefix: prefix, indent: indent})
	if err != nil {
		return nil, err
	}
	buf := append([]byte(nil), e.Bytes()...)

	return buf, nil
}

// An encodeState encodes JSON into a bytes.Buffer.
type specEncodeState struct {
	bytes.Buffer // accumulated output

	// Keep track of what pointers we've seen in the current recursive call
	// path, to avoid cycles that could lead to a stack overflow. Only do
	// the relatively expensive map operations if ptrLevel is larger than
	// startDetectingCyclesAfter, so that we skip the work if we're within a
	// reasonable amount of nested pointers deep.
	ptrLevel uint
	ptrSeen  map[any]struct{}
}

var specEncodeStatePool sync.Pool

func newSpecEncodeState() *specEncodeState {
	if v := specEncodeStatePool.Get(); v != nil {
		e := v.(*specEncodeState)
		e.Reset()
		if len(e.ptrSeen) > 0 {
			panic("ptrEncoder.encode should have emptied ptrSeen via defers")
		}
		e.ptrLevel = 0
		return e
	}
	return &specEncodeState{ptrSeen: make(map[any]struct{})}
}

type jsonError struct{ error }

func (e *specEncodeState) marshal(t reflect.Type, opts encOpts) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if je, ok := r.(jsonError); ok {
				err = je.error
			} else {
				panic(r)
			}
		}
	}()
	e.reflectType(t, opts)
	return nil
}

// error aborts the encoding by panicking with err wrapped in jsonError.
func (e *specEncodeState) error(err error) {
	panic(jsonError{err})
}

func (e *specEncodeState) reflectType(t reflect.Type, opts encOpts) {
	specEncoder(t)(e, opts)
}

type specEncoderFunc func(e *specEncodeState, opts encOpts)

var specEncoderCache sync.Map // map[reflect.Type]encoderFunc

func specEncoder(t reflect.Type) specEncoderFunc {
	return typeSpecEncoder(t)
}

func typeSpecEncoder(t reflect.Type) specEncoderFunc {
	if fi, ok := specEncoderCache.Load(t); ok {
		return fi.(specEncoderFunc)
	}

	// To deal with recursive types, populate the map with an
	// indirect func before we build it. This type waits on the
	// real func (f) to be ready and then calls it. This indirect
	// func is only used for recursive types.
	var (
		wg sync.WaitGroup
		f  specEncoderFunc
	)
	wg.Add(1)
	fi, loaded := specEncoderCache.LoadOrStore(t, specEncoderFunc(func(e *specEncodeState, opts encOpts) {
		wg.Wait()
		if f == nil {
			e.error(fmt.Errorf("recursive encoder is nil for type %v", t))
			return
		}
		f(e, opts)
	}))

	if loaded {
		return fi.(specEncoderFunc)
	}

	// Compute the real encoder and replace the indirect func with it.
	f = newTypeSpecEncoder(t)
	wg.Done()
	specEncoderCache.Store(t, f)
	return f
}

// newTypeSpecEncoder constructs an encoderFunc for a type.
// The returned encoder only checks CanAddr when allowAddr is true.
func newTypeSpecEncoder(t reflect.Type) specEncoderFunc {
	switch t.Kind() {
	case reflect.Struct:
		return newStructSpecEncoder(t)
	case reflect.Array, reflect.Slice:
		if t.Elem().Kind() == reflect.Struct {
			return newArraySpecEncoder(newTypeSpecEncoder(t.Elem()))
		}
	}
	return nil
}
