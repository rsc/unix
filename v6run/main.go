// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"io"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"golang.org/x/term"
	"rsc.io/unix/v6unix"
)

var (
	trace      = flag.Bool("trace", false, "trace every instruction")
	cpuprofile = flag.String("cpuprofile", "", "write cpuprofile to `file`")
)

func main() {
	log.SetPrefix("v6run: ")
	log.SetFlags(0)
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	fixup := func() { term.Restore(int(os.Stdin.Fd()), oldState) }
	defer fixup()

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
				if c == 0x1c {
					pprof.StopCPUProfile()
					fixup()
					os.Exit(0)
				}
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
