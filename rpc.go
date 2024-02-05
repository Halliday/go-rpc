package rpc

//
// What is the cost of lies?

import (
	"context"
	_ "embed"
	"net/url"
)

var ctxKey struct{}

type Context interface {
	context.Context
	Procedure() *Procedure
	RemoteAddr() string
	URL() *url.URL
}

func FindContext(ctx context.Context) Context {
	if ctx == nil {
		return nil
	}
	val, _ := ctx.Value(ctxKey).(Context)
	return val
}
