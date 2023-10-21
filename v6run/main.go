// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"io"
	"log"
	"os"
	"time"

	v6unix "rsc.io/unix/v6unix"
)

var trace = flag.Bool("trace", false, "trace every instruction")

func main() {
	log.SetPrefix("v6run: ")
	log.SetFlags(0)
	flag.Parse()

	sys, err := v6unix.NewSystem(v6unix.FS)
	if err != nil {
		log.Fatal(err)
	}
	sys.Trace = *trace

	aout, err := sys.ReadFile("/etc/init")
	if err != nil {
		log.Fatal(err)
	}

	_, err = sys.Start(aout, []string{"/etc/init"}, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	input := make(chan byte, 1000)
	go func() {
		buf := make([]byte, 100)
		defer close(input)
		for {
			n, err := os.Stdin.Read(buf)
			for _, c := range buf[:n] {
				input <- c
			}
			if err == io.EOF {
				input <- 0o004
			} else if err != nil {
				log.Fatalf("reading stdin: %v", err)
			}
		}
	}()

	for {
		sys.Wait()
		var c1 chan byte
		if sys.TTYRead != 0 {
			c1 = input
		}
		var c2 <-chan time.Time
		if !sys.Timer.IsZero() {
			c2 = time.After(time.Until(sys.Timer))
		}
		if c1 == nil && c2 == nil {
			break
		}

		select {
		case b := <-c1:
			if b == 0 {
				sys.TTY[8].EOF = true
			} else {
				sys.TTY[8].WriteByte(b)
			}
		Loop:
			for {
				select {
				default:
					break Loop
				case b := <-c1:
					if b == 0 {
						sys.TTY[8].EOF = true
					} else {
						sys.TTY[8].WriteByte(b)
					}
				}
			}
		case <-c2:
			// timer went off; sys.Wait will notice
		}
	}
}
