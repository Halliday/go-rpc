package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/halliday/go-tools/httptools"
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
			httptools.ServeError(resp, req, err)
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
		httptools.ServeError(resp, req, err)
		return
	}

	if output != nil {
		if str, ok := output.(string); ok {
			resp.Write([]byte(str))
			return
		}

		data, err := json.Marshal(output)
		if err != nil {
			httptools.ServeError(resp, req, err)
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

func (ctx *HTTPContext) Value(key interface{}) interface{} {
	if key == ctxKey {
		return context.Context(ctx)
	}
	return ctx.Context.Value(key)
}
