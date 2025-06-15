package jsonspec

import (
	"reflect"
	"sync"
)

// A field represents a single field found in a struct.
type specField struct {
	typ reflect.Type

	name      string
	nameBytes []byte // []byte(name)

	nameNonEsc  string // `"` + name + `":`
	nameEscHTML string // `"` + HTMLEscape(name) + `":`

	spec string

	tag bool

	encoder specEncoderFunc
}

// typeFields returns a list of fields that JSON should recognize for the given type.
// The algorithm is breadth-first search over the set of structs to include - the top struct
// and then any reachable anonymous structs.
//
// typeFields should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/bytedance/sonic
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
func typeSpecFields(t reflect.Type) structSpecFields {
	// Anonymous fields to explore at the current level and the next.
	current := []specField{}
	next := []specField{{typ: t}}

	// Count of queued names for current level and the next.
	var count, nextCount map[reflect.Type]int

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}

	// Fields found.
	var fields []specField

	// Buffer to run appendHTMLEscape on field names.
	var nameEscBuf []byte

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}

		for _, f := range current {
			if visited[f.typ] {
				continue
			}
			visited[f.typ] = true

			// Scan f.typ for fields to include.
			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				if sf.Anonymous {
					t := sf.Type
					if t.Kind() == reflect.Pointer {
						t = t.Elem()
					}
					if !sf.IsExported() && t.Kind() != reflect.Struct {
						// Ignore embedded fields of unexported non-struct types.
						continue
					}
					// Do not ignore embedded fields of unexported struct types
					// since they may have exported fields.
				} else if !sf.IsExported() {
					// Ignore unexported non-embedded fields.
					continue
				}

				tag := sf.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, _ := parseTag(tag)
				if !isValidTag(name) {
					name = ""
				}
				spec := sf.Tag.Get("spec")

				// index := make([]int, len(f.index)+1)
				// copy(index, f.index)
				// index[len(f.index)] = i

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Pointer {
					// Follow pointer.
					ft = ft.Elem()
				}

				// // Only strings, floats, integers, and booleans can be quoted.
				// // quoted := false
				// if opts.Contains("string") {
				// 	switch ft.Kind() {
				// 	case reflect.Bool,
				// 		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				// 		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
				// 		reflect.Float32, reflect.Float64,
				// 		reflect.String:
				// 		// quoted = true
				// 	}
				// }

				// Record found field and index sequence.
				if name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
					tagged := name != ""
					if name == "" {
						name = sf.Name
					}
					field := specField{
						name: name,
						tag:  tagged,
						typ:  ft,
						spec: spec,
					}
					field.nameBytes = []byte(field.name)

					// Build nameEscHTML and nameNonEsc ahead of time.
					nameEscBuf = appendHTMLEscape(nameEscBuf[:0], field.nameBytes)
					field.nameEscHTML = `"` + string(nameEscBuf) + `":`
					field.nameNonEsc = `"` + field.name + `":`

					fields = append(fields, field)
					if count[f.typ] > 1 {
						// If there were multiple instances, add a second,
						// so that the annihilation code will see a duplicate.
						// It only cares about the distinction between 1 and 2,
						// so don't bother generating any more copies.
						fields = append(fields, fields[len(fields)-1])
					}
					continue
				}

				// Record new anonymous struct to explore in next round.
				nextCount[ft]++
				if nextCount[ft] == 1 {
					next = append(next, specField{name: ft.Name(), typ: ft})
				}
			}
		}
	}

	for i := range fields {
		f := &fields[i]
		if f.typ.Kind() == reflect.Struct {
			f.encoder = newStructSpecEncoder(f.typ)
		}

		if f.typ.Kind() == reflect.Slice && f.typ.Elem().Kind() == reflect.Struct {
			f.encoder = newArraySpecEncoder(newStructSpecEncoder(f.typ.Elem()))
		}
	}

	return structSpecFields{fields}
}

var specFieldCache sync.Map // map[reflect.Type]structFields

// cachedTypeFields is like typeFields but uses a cache to avoid repeated work.
func cachedSpecTypeFields(t reflect.Type) structSpecFields {
	if f, ok := specFieldCache.Load(t); ok {
		return f.(structSpecFields)
	}
	f, _ := specFieldCache.LoadOrStore(t, typeSpecFields(t))
	return f.(structSpecFields)
}
