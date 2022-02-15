package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/gopherd/tools/cmd/gopherlint/final"
	"github.com/gopherd/tools/cmd/gopherlint/unusedresult"
)

func main() {
	multichecker.Main(
		unusedresult.Analyzer,
		final.Analyzer,
	)
}
