package main

import (
	_ "net/http/pprof"

	"obscure-fs-rebuild/cmd"
)

func main() {
	cmd.Execute()
}
