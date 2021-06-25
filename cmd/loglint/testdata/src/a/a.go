package a

import "github.com/gopherd/log"

func _() {
	// wrong
	log.Debug()                                    // want: result of github.com/gopherd/log.Debug call not used
	log.Prefix("pre").Trace()                      // want: result of (github.com/gopherd/log.Prefix).Trace call not used
	log.Debug().String("key", "value")             // want: result of (*github.com/gopherd/log.Fields).String call not used
	log.Debug().Int("i", 1).String("key", "value") // want: result of (*github.com/gopherd/log.Fields).String call not used

	// right
	log.Debug().String("key", "value").Print("message")
	log.Debug().Print("message")
}
