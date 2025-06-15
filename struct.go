package jsonspec

import "reflect"

type structSpecEncoder struct {
	fields structSpecFields
}

type structSpecFields struct {
	list []specField
	// byExactName  map[string]*specField
	// byFoldedName map[string]*specField
}

func (se structSpecEncoder) encode(e *specEncodeState, opts encOpts) {
	next := byte('{')

	for i := range se.fields.list {
		f := &se.fields.list[i]

		if next == '{' {
			e.WriteString(opts.prefix)
		}

		e.WriteByte(next)
		e.WriteByte('\n')
		next = ','

		e.WriteString(opts.prefix)
		if opts.escapeHTML {
			e.WriteString(f.nameEscHTML)
		} else {
			e.WriteString(f.nameNonEsc)
		}

		if f.encoder == nil {
			e.WriteString(f.spec)
		} else {
			nOpt := opts
			nOpt.prefix = opts.prefix + opts.indent
			f.encoder(e, nOpt)
		}
	}

	if next == '{' {
		e.WriteString("{}")
	} else {
		e.WriteByte('\n')
		e.WriteString(opts.prefix)
		e.WriteByte('}')
	}
}

func newStructSpecEncoder(t reflect.Type) specEncoderFunc {
	se := structSpecEncoder{fields: cachedSpecTypeFields(t)}
	return se.encode
}
