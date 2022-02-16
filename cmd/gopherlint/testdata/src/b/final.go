package b

//@mod:final
var Final = 1

type UserInfo struct {
	name string
}

func (user *UserInfo) Reset() {
	*user = UserInfo{name: "noname"}
}

func (_ *UserInfo) UnderscoreReceiver() string {
	return "UserInfo"
}

func (*UserInfo) EmptyReceiver() int {
	return 0
}

func (user UserInfo) NonPointerReceiver() int {
	return 0
}

//@mod:final
var User = UserInfo{name: "hello"}

//@mod:final
var UserPtr = &UserInfo{name: "hello"}

var otherUser = UserInfo{name: "world"}

func ResetUser(name string) {
	// can't assign a value to final variable User
	User = UserInfo{name: name}
}

func _() {
	// can't reference final variable User
	User.Reset()

	// It's ok
	UserPtr.Reset()

	// It's ok
	User.UnderscoreReceiver()

	// It's ok
	User.EmptyReceiver()

	// It's ok
	User.NonPointerReceiver()

	// It's ok
	otherUser.Reset()
}
