package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/halliday/go-values"
)

var ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()
var errorType = reflect.TypeOf((*error)(nil)).Elem()

type Procedure struct {
	in  reflect.Type
	out reflect.Type

	fn reflect.Value

	middleware []func(next http.Handler) http.Handler
}

func (p Procedure) InputType() reflect.Type {
	return p.in
}

func (p Procedure) OutputType() reflect.Type {
	return p.out
}

func New(fn interface{}, opts ...Option) (*Procedure, error) {

	e := new(Procedure)
	e.fn = reflect.ValueOf(fn)
	t := e.fn.Type()
	if t.Kind() != reflect.Func {
		return e, fmt.Errorf("called with non func: %v", t)
	}

	switch t.NumIn() {
	case 2:
		e.in = t.In(1)
		fallthrough
	case 1:
		if t.In(0) != ctxType {
			return e, fmt.Errorf("input 0 must be type 'context.Context' but is '%s'", t.In(0))
		}
	default:
		return e, fmt.Errorf("expected func with 1 .. 2 inputs")
	}

	switch t.NumOut() {
	case 2:
		e.out = t.In(0)
		fallthrough
	case 1:
		if t.Out(t.NumOut()-1) != errorType {
			return e, fmt.Errorf("output 0 must be type 'error' but is '%s'", t.Out(t.NumOut()-1))
		}
	default:
		return e, fmt.Errorf("expected func with 1 .. 2 outputs")
	}

	if opts == nil {
		opts = DefaultOptions
	}
	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(e); err != nil {
				return e, err
			}
		}
	}

	return e, nil
}

func MustNew(fn interface{}, opts ...Option) *Procedure {
	e, err := New(fn, opts...)
	if err != nil {
		panic(err)
	}
	return e
}

func (p *Procedure) Call(ctx context.Context, input interface{}) (output interface{}, err error) {
	var in [3]reflect.Value
	in[0] = reflect.ValueOf(ctx)
	numIn := 1
	if p.in != nil {
		in[1] = reflect.ValueOf(input)
		numIn = 2
	}
	out := p.fn.Call(in[:numIn])
	if p.out != nil {
		output = out[0].Interface()
		err, _ = out[1].Interface().(error)
	} else {
		err, _ = out[0].Interface().(error)
	}
	return output, err
}

func UnmarshalRequest(req *http.Request, v interface{}) error {
	if err := req.ParseForm(); err != nil {
		return err
	}

	if err := values.Unmarshal(req.Form, v); err != nil {
		return fmt.Errorf("can not unmarshal request values: %w", err)
	}

	switch req.Header.Get("Content-Type") {
	case "application/json":
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("can not unmarshal request values: %w", err)
		}
		return nil

	case "application/x-www-form-urlencoded":
		// already handled by above `values.Unmarshal()`
		return nil

	case "":
		// no body content
		return nil
	default:
		return fmt.Errorf("unsupported 'Content-Type' header: %q", req.Header.Get("Content-Type"))
	}
}
