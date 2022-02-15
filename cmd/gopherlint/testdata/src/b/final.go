package b

//@mod:final
var Final = 1

type UserInfo struct {
	name string
}

func (user *UserInfo) Reset() {
	*user = UserInfo{name: "noname"}
}

//@mod:final
var User = UserInfo{name: "hello"}

//@mod:final
var UserPtr = &UserInfo{name: "hello"}

var otherUser = UserInfo{name: "world"}

func ResetUser(name string) {
	User = UserInfo{name: name}
}

func _() {
	// can't reference final variable User
	User.Reset()

	// It's ok
	otherUser.Reset()

	// It's ok
	UserPtr.Reset()
}
