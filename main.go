// SPDX-License-Identifier: BSD-3-Clause
//
// Command app-template is the go-widgets application starter: a tiny but
// real browser wasm canvas app that renders a filterable list with
// go-widgets/toolkit, holds ALL state in go-widgets/mvvm, binds the two
// through go-widgets/mvvmtk, and is CI-gated by go-widgets/mvvmlint from
// commit #1. Copy it, rename the module, and grow your own VM + View.
//
// This file is the js/wasm canvas driver ONLY: it forwards mouse / wheel /
// keyboard input into state.handle*, calls state.draw into an ImageData,
// and CopyBytesToJS -> putImageData. It carries the js && wasm build tag
// and is excluded from coverage; the ViewModel (viewmodel.go) and the View
// (scene.go) are tagless and fully native-testable, mirroring the
// go-widgets/gallery and go-pkgx/registry-viewer templates.
//
//go:build js && wasm

package main

import (
	"syscall/js"
)

func main() {
	doc := js.Global().Get("document")
	canvas := doc.Call("getElementById", "screen")
	if canvas.IsUndefined() || canvas.IsNull() {
		println("app-template: no #screen canvas in the host page")
		return
	}
	canvas.Set("width", surfaceW)
	canvas.Set("height", surfaceH)
	ctx := canvas.Call("getContext", "2d")

	local := make([]byte, 4*surfaceW*surfaceH)
	imageData := ctx.Call("createImageData", surfaceW, surfaceH)
	dst := imageData.Get("data")

	state := newState(surfaceW, surfaceH)

	render := func() {
		state.draw(local)
		js.CopyBytesToJS(dst, local)
		ctx.Call("putImageData", imageData, 0, 0)
	}

	// Mouse dispatch: convert clientX/Y to canvas-local pixel coords via the
	// bounding rect (accounting for any CSS scaling), then route.
	coords := func(e js.Value) (int, int) {
		rect := canvas.Call("getBoundingClientRect")
		sx := rect.Get("width").Float() / float64(surfaceW)
		sy := rect.Get("height").Float() / float64(surfaceH)
		x := int((e.Get("clientX").Float() - rect.Get("left").Float()) / sx)
		y := int((e.Get("clientY").Float() - rect.Get("top").Float()) / sy)
		return x, y
	}
	listen := func(name string, handle func(x, y int) bool) {
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			x, y := coords(args[0])
			if handle(x, y) {
				render()
			}
			return nil
		})
		canvas.Call("addEventListener", name, cb)
	}
	canvas.Call("addEventListener", "mousedown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 || args[0].Get("button").Int() != 0 {
			return nil
		}
		x, y := coords(args[0])
		if state.handleClick(x, y) {
			render()
		}
		return nil
	}))
	listen("mousemove", func(x, y int) bool { return state.handleMove(x, y) })
	listen("mouseup", func(x, y int) bool { return state.handleRelease(x, y) })

	// Wheel: forward the browser scroll to the widget under the pointer as a
	// toolkit EventScroll, Delta expressed in ROWS. The scrollable widget
	// does all the offset math + clamping. preventDefault stops the page
	// from scrolling under us.
	canvas.Call("addEventListener", "wheel", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		e := args[0]
		e.Call("preventDefault")
		x, y := coords(e)
		d := 3
		if e.Get("deltaY").Float() < 0 {
			d = -3
		}
		if state.handleScroll(x, y, d) {
			render()
		}
		return nil
	}), map[string]any{"passive": false})

	// Keyboard: a single printable rune with no modifier is text (EventChar)
	// for the focused SearchEntry; every named key is an EventKeyDown. The
	// window listens because a <canvas> is not focusable by default.
	js.Global().Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		e := args[0]
		key := e.Get("key").String()
		var changed bool
		if len([]rune(key)) == 1 && !e.Get("ctrlKey").Bool() && !e.Get("metaKey").Bool() && !e.Get("altKey").Bool() {
			changed = state.handleChar(key)
		} else {
			changed = state.handleKeyDown(key)
		}
		if changed {
			e.Call("preventDefault")
			render()
		}
		return nil
	}))

	render()
	select {} // park forever so the callbacks live
}
