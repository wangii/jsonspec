package jsonspec

type arraySpecEncoder struct {
	nestedEncoder specEncoderFunc
}

func newArraySpecEncoder(nested specEncoderFunc) specEncoderFunc {
	return arraySpecEncoder{nested}.encode
}

func (a arraySpecEncoder) encode(e *specEncodeState, opts encOpts) {
	e.WriteString("[\n")
	nopts := opts
	nopts.prefix = opts.prefix + opts.indent
	a.nestedEncoder(e, nopts)

	e.WriteString(",...]")
}
