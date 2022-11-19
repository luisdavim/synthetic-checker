//go:build exclude

package main

import (
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/luisdavim/synthetic-checker/cmd"
)

// genDocs is a helper function to generate the tool's usage documentation
func genDocs(docsPath string) {
	rootCmd := cmd.NewCmd(nil)

	if err := doc.GenMarkdownTree(rootCmd, docsPath); err != nil {
		os.Exit(1)
	}
}

func main() {
	path := "./usage"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if err := os.MkdirAll(path, 0o775); err != nil {
		panic(err)
	}
	genDocs(path)
}
