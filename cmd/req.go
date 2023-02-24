package cmd

type req struct {
	methodName string
	in         []interface{}
	err        chan error

	// option
	resultChecker      resultCheckFunc
	expectCallAPIError bool
}

type reqOpt func(*req)

func newReq(methodName string, in []interface{}, opts ...reqOpt) *req {
	r := &req{
		methodName: methodName,
		in:         in,
		err:        make(chan error, 1),
	}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func withResultCheck(f resultCheckFunc) reqOpt {
	return func(r *req) {
		r.resultChecker = f
	}
}

func withExpectCallAPIError() reqOpt {
	return func(r *req) {
		r.expectCallAPIError = true
	}
}

type resultCheckFunc func(r1, r2 interface{}) error
