//go:build exclude

package main

import (
	"os"

	"github.com/luisdavim/synthetic-checker/cmd"
)

func main() {
	path := "./usage"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if err := os.MkdirAll(path, 0o775); err != nil {
		panic(err)
	}
	cmd.GenDocs(path)
}
