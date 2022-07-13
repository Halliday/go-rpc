package rpc

import (
	_ "embed"

	"github.com/halliday/go-module"
)

//go:embed messages.csv
var messages string

var _, e, Module = module.New("rpc", messages)
