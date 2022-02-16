package b

//@mod:final
var Final = 1

type UserInfo struct {
	Id   int
	Info struct {
		Name string
	}
}

func (user *UserInfo) Reset() {
	*user = UserInfo{}
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
var User = UserInfo{Id: 1}

//@mod:final
var UserPtr = &UserInfo{Id: 2}

var otherUser = UserInfo{Id: 3}

func ResetUser(id int) {
	// cannot assign a value to final variable User
	User = UserInfo{Id: id}
}

func _() {
	// cannot reference final variable User
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

	// cannot assign a value to final variable User
	User.Id = 0

	// cannot assign a value to field of final variable User
	User.Info.Name = "new name"

	// cannot reference field of final variable User
	var nameptr = &User.Info.Name
	_ = nameptr
}
