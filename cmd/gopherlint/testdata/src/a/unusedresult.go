package a

type user struct {
}

func (u *user) self() *user { return u }
func (u *user) end()        {}

func _() {
	var u user

	// expect warning: result of (github.com/gopherd/tools/cmd/gopherlint/testdata/src/a.user).self call not used
	u.self()
}
