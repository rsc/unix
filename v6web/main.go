// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate cp $GOROOT/misc/wasm/wasm_exec.js .
//go:generate env GOOS=js GOARCH=wasm go build -o main.wasm

package main

import (
	"fmt"
	"html"
	"log"
	"os"
	"strings"
	"syscall/js"
	"time"

	v6unix "rsc.io/unix/v6unix"
)

func fatal(err error) {
	log.Fatal(err)
}

var (
	doc    js.Value
	input  js.Value
	bottom js.Value
	ttyall js.Value
	curtty int
)

// TODO F1 F2 F3 F4 F5 F6 F7 F8 for console switching

const cursor = "‸"

func tprint(tty int) func(b []byte, echo bool) (int, v6unix.Errno) {
	return func(b []byte, echo bool) (int, v6unix.Errno) {
		text := html.EscapeString(string(b))
		text = strings.ReplaceAll(text, "\003", "^C")
		text = strings.ReplaceAll(text, "\004", "^D")
		if echo {
			text = "<b>" + text + "</b>"
		}
		output := doc.Call("getElementById", fmt.Sprintf("tty%d", tty))
		inner := strings.TrimSuffix(output.Get("innerHTML").String(), cursor)
		if strings.HasSuffix(inner, "</b>") && strings.HasPrefix(text, "<b>") {
			inner = strings.TrimSuffix(inner, "</b>")
			text = strings.TrimPrefix(text, "<b>")
		}
		output.Set("innerHTML", js.ValueOf(inner+text+cursor))
		bottom.Call("scrollIntoView", js.ValueOf(false))
		return len(b), 0
	}
}

func main() {
	sys, err := v6unix.NewSystem(v6unix.FS)
	if err != nil {
		fatal(err)
	}

	aout, err := sys.ReadFile("/etc/init")
	if err != nil {
		fatal(err)
	}

	doc = js.Global().Get("document")
	input = doc.Call("getElementById", "input")
	bottom = doc.Call("getElementById", "bottom")
	ttyall = doc.Call("getElementById", "ttyall")

	if _, err := sys.Start(aout, []string{"/etc/init"}, os.Stdout); err != nil {
		fatal(err)
	}

	ttys := []int{0, 1, 2, 3, 8}

	setTTY := func(i int) {
		curtty = i
		for _, j := range ttys {
			name := fmt.Sprintf("tty%d", j)
			tty := doc.Call("getElementById", name)
			button := doc.Call("getElementById", "b"+name)
			if tty.IsNull() || button.IsNull() {
				continue
			}
			if i == j {
				tty.Get("style").Set("display", "block")
				button.Get("style").Set("background-color", "blue")
				button.Get("style").Set("color", "white")
			} else {
				tty.Get("style").Set("display", "none")
				button.Get("style").Set("background-color", "#e0e0e0")
				button.Get("style").Set("color", "black")
			}
		}
		input.Call("focus")
	}
	setTTY(8)
	for _, i := range ttys {
		sys.TTY[i].Print = tprint(i)
		i := i
		doc.Call("getElementById", fmt.Sprintf("btty%d", i)).Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) any { setTTY(i); return nil }))
	}
	ttyall.Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) any {
		input.Call("focus")
		return nil
	}))

	// element.classList.remove("mystyle");
	// document.elm.style.border = "3px solid #FF0000";

	ready := make(chan bool, 1)

	wakeup := func() {
		select {
		case ready <- true:
		default:
		}
	}

	keydown := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		e.Call("preventDefault")
		key := e.Get("key").String()
		ctrl := e.Get("ctrlKey").Bool()
		shift := e.Get("shiftKey").Bool()
		switch key {
		default:
			if len(key) > 1 {
				return nil
			}
		case "Enter":
			key = "\n"
		case "Backspace":
			key = "\b"
		case "Escape":
			key = "\033"
		case "Tab":
			key = "\t"
		}
		c := key[0]
		if (shift || ctrl) && 'a' <= c && c <= 'z' {
			c += ('A' - 'a') & 0o377
		}
		if ctrl && c >= '@' {
			c -= '@'
		}
		sys.TTY[curtty].WriteByte(c)
		wakeup()
		return nil
	})

	change := js.FuncOf(func(this js.Value, args []js.Value) any {
		v := input.Get("value").String()
		for _, b := range []byte(v) {
			sys.TTY[curtty].WriteByte(b)
			wakeup()
		}
		input.Set("value", "")
		return nil
	})

	input.Call("addEventListener", "keydown", keydown)
	input.Call("addEventListener", "input", change)
	input.Call("focus")

	fmt.Printf("started\n")

	var timer *time.Timer
	var lastTimer time.Time
	for {
		sys.Wait()
		if !sys.Timer.IsZero() && (lastTimer.IsZero() || lastTimer.After(sys.Timer)) {
			d := time.Until(sys.Timer)
			if timer == nil {
				timer = time.AfterFunc(d, wakeup)
			} else {
				timer.Reset(d)
			}
		}
		<-ready
	}

	select {}
}
