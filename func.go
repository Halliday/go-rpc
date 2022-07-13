package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	"github.com/halliday/go-tools"
	"github.com/halliday/go-values"
)

var ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()
var errorType = reflect.TypeOf((*error)(nil)).Elem()

type Procedure struct {
	in  reflect.Type
	out reflect.Type

	fn reflect.Value
}

func (p Procedure) InputType() reflect.Type {
	return p.in
}

func (p Procedure) OutputType() reflect.Type {
	return p.out
}

func New(fn interface{}) (*Procedure, error) {
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
			return e, fmt.Errorf("output 0 must be type 'error' but is '%s'", t.In(t.NumOut()-1))
		}
	default:
		return e, fmt.Errorf("expected func with 1 .. 2 outputs")
	}

	return e, nil
}

func MustNew(fn interface{}) *Procedure {
	e, err := New(fn)
	if err != nil {
		panic(fmt.Errorf("NewProcedure: %v", err))
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

func (p *Procedure) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodOptions {
		return
	}

	path := req.URL.Path
	if path != "/" && path != "" {
		http.NotFound(resp, req)
		return
	}

	var input interface{}
	if p.in != nil {
		var q reflect.Value
		if p.in.Kind() == reflect.Ptr {
			q = reflect.New(p.in.Elem())
		} else {
			q = reflect.New(p.in)
		}

		err := UnmarshalRequest(req, q.Interface())
		if err != nil {
			tools.ServeError(resp, err)
			return
		}

		if p.in.Kind() == reflect.Ptr {
			input = q.Interface()
		} else {
			input = q.Elem().Interface()
		}
	}

	output, err := p.Call(req.Context(), input)

	if err != nil {
		if redirect, ok := err.(*redirect); ok {
			http.Redirect(resp, req, redirect.url, redirect.code)
			return
		}
		tools.ServeError(resp, err)
		return
	}

	if output != nil {
		if str, ok := output.(string); ok {
			resp.Write([]byte(str))
			return
		}

		data, err := json.Marshal(output)
		if err != nil {
			tools.ServeError(resp, err)
			return
		}

		resp.Header().Set("Content-Type", "application/json")
		resp.Write(data)
	}
}

type redirect struct {
	url  string
	code int
}

func (err redirect) Error() string {
	return "redirect"
}

func (err redirect) ErrorCode() int {
	return err.code
}

func Redirect(url string, code int) error {
	return &redirect{url, code}
}

func UnmarshalRequest(req *http.Request, v interface{}) error {
	if err := req.ParseForm(); err != nil {
		return err
	}

	if err := values.Unmarshal(req.Form, v); err != nil {
		return e("values_parse", err)
	}

	switch req.Header.Get("Content-Type") {
	case "application/json":
		data, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, v); err != nil {
			return e("json_unmarshal", err)
		}
		return nil

	case "application/x-www-form-urlencoded":
		// already handled by above `values.Unmarshal()`
		return nil

	case "":
		// no body content
		return nil
	default:
		return ErrContentType
	}
}

var ErrContentType = e("unsupported_content_type")
