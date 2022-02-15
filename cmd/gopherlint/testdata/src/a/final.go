package a

import (
	"github.com/gopherd/tools/cmd/gopherlint/testdata/src/b"
)

//@mod:final
var finalSingle = 1

// ....
//
//@mod:final
//
// balabala ...
var (
	finalGroup1 = "hello"
	finalGroup2 = "world"
	_           = "xxx"
)

func _() {
	b.Final = 2

	finalSingle = 2

	finalGroup1 = "h"
	finalGroup2 = "w"
	_ = "x"

	//@mod:final
	var x int
	x = 2
	_ = x
}
