package main

import (
	"github.com/gopherjs/gopherjs/js"
)

func main() {
	js.Global.Set("ot", map[string]interface{}{
		"NewSequence": func() *js.Object {
			return js.MakeFullWrapper(NewSequence())
		},
		"FromString": func(str string) *js.Object {
			return js.MakeFullWrapper(FromString(str))
		},
	})
}
