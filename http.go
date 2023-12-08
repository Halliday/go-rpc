package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"

	"github.com/halliday/go-errors"
)

const AllowHeaders = "Content-Type"

func (p *Procedure) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodOptions {
		requestHeaders := req.Header.Get("Access-Control-Request-Headers")
		if requestHeaders != "" {
			allowHeaders := resp.Header().Get("Access-Control-Allow-Headers")
			if allowHeaders != "" {
				allowHeaders += ", " + AllowHeaders
			} else {
				allowHeaders = AllowHeaders
			}
			resp.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		}
		return
	}
	if req.Method == http.MethodHead {
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
			serveError(resp, err)
			return
		}

		if p.in.Kind() == reflect.Ptr {
			input = q.Interface()
		} else {
			input = q.Elem().Interface()
		}
	}

	ctx := &HTTPContext{
		Context:   req.Context(),
		procedure: p,
		Request:   req,
		Response:  resp,
	}
	output, err := p.Call(ctx, input)

	if err != nil {
		serveError(resp, err)
		return
	}

	if output != nil {
		if str, ok := output.(string); ok {
			resp.Write([]byte(str))
			return
		}

		data, err := json.Marshal(output)
		if err != nil {
			serveError(resp, err)
			return
		}

		resp.Header().Set("Content-Type", "application/json")
		resp.Write(data)
	}
}

type HTTPContext struct {
	context.Context
	procedure *Procedure
	Response  http.ResponseWriter
	Request   *http.Request
}

var _ Context = (*HTTPContext)(nil)

func (ctx HTTPContext) Procedure() *Procedure {
	return ctx.procedure
}

func (ctx HTTPContext) RemoteAddr() string {
	return ctx.Request.RemoteAddr
}

func (ctx *HTTPContext) Value(key interface{}) interface{} {
	if key == ctxKey {
		return context.Context(ctx)
	}
	return ctx.Context.Value(key)
}

var ErrInternal = errors.NewRich("internal server error", 500, "internal server error", "", nil, nil)

func serveError(resp http.ResponseWriter, err error) {
	safe, _ := errors.Safe(err)
	resp.Header().Set("X-Content-Type-Options", "nosniff")
	if safe != nil {
		code := range1000(safe.(*errors.RichError).Code)
		serveJSON(resp, code, safe)
	} else {
		serveJSON(resp, 500, ErrInternal)
		Logger.Printf("rpc: unsafe error: %v", err)
	}
}

func range1000(code int) int {
	for code > 1000 {
		code /= 10
	}
	return code
}

func serveJSON(resp http.ResponseWriter, code int, data any) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(data)
	if err != nil {
		Logger.Printf("rpc: json encoder error %#v: %v", data, err)
		serveError(resp, err)
		return
	}
	resp.Header().Set("Content-Type", "application/json")
	resp.Header().Set("Content-Length", strconv.Itoa(b.Len()))
	resp.WriteHeader(code)
	resp.Write(b.Bytes())
}
