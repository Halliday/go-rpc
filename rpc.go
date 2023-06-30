package rpc

//
// What is the cost of lies?

import (
	"context"
	_ "embed"
	"log"

	"github.com/halliday/go-module"
)

//go:embed messages.csv
var messages string

var _, e, Module = module.New("rpc", messages)

//

var Logger = log.Default()

//

var ctxKey struct{}

type Context interface {
	context.Context
	Procedure() *Procedure
	RemoteAddr() string
}

func FindContext(ctx context.Context) Context {
	if ctx == nil {
		return nil
	}
	val, _ := ctx.Value(ctxKey).(Context)
	return val
}
