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
	// Error: cannot assign a value to final variable Final
	b.Final = 2

	// Error: cannot assign a value to final variable finalSingle
	finalSingle = 2

	// Error: cannot assign a value to final variable finalGroup1
	finalGroup1 = "h"
	// Error: cannot assign a value to final variable finalGroup2
	finalGroup2 = "w"
	_ = "x"

	//@mod:final
	//
	var x int
	// Error: cannot assign a value to final variable x
	x = 2
	// Error: cannot reference final variable x
	unused(&x)

	var z int //@mod:final
	// It's ok because of @mod:final must be a document comment instead of line comment
	z = 1
	unused(z)

	// cannot reference final variable User
	b.User.Reset()

	// It's ok
	b.UserPtr.Reset()

	//@mod:final
	var slice = []int{1, 2}
	// It's ok
	slice[0] = 2
	// cannot reference final variable slice
	unused(&slice)

	//@mod:final
	var users = []b.UserInfo{
		{Id: 1},
	}
	// It's ok
	users[0].Id = 2
}

func unused(a ...interface{}) {}
