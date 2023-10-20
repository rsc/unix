// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func Asm(pc uint16, text string) (codes []uint16, err error) {
	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(runtime.Error); ok {
				panic(e)
			}
			err = fmt.Errorf("asm %q: %v", text, e)
		}
	}()

	op, args := parseAsm(text)
	inst := lookupAsm(op)
	if inst == nil {
		panic("unknown instruction")
	}
	_, iargs := parseAsm(inst.text)
	if len(args) != len(iargs) {
		panic(fmt.Sprintf("invalid argument count %d != %d", len(args), len(iargs)))
	}

	out := []uint16{inst.code}
	for i, arg := range args {
		switch iarg := iargs[i]; iarg {
		case "%b": // branch offset
			n := parseConst(arg)
			d := int16(n-(pc+2)) / 2
			if d != int16(int8(d)) {
				panic("branch target out of range")
			}
			out[0] |= uint16(d) & 0o377
		case "%B": // sob offset
			n := parseConst(arg)
			d := int16(n-(pc+2)) / 2
			if d > 0 || d < -2*0o77 {
				panic("branch target out of range")
			}
			out[0] |= uint16(-d) & 0o77
		case "%n": // emt/trap number
			n := parseConst(arg)
			if n != n&0o377 {
				panic("emt/trap number out of range")
			}
			out[0] |= n
		case "%r": // register number at bit 6
			out[0] |= uint16(parseReg(arg)) << 6
		case "%R": // register number at bit 0
			out[0] |= uint16(parseReg(arg)) << 0
		case "%d": // destination
			out = parseArg(pc, arg, 0, false, out)
		case "%s": // source
			out = parseArg(pc, arg, 6, false, out)
		case "%f": // fdst/fsrc
			out = parseArg(pc, arg, 0, true, out)
		case "%a": // accumulator index
			out[0] |= parseAC(arg)
		}
	}
	return out, nil
}

func parseAC(arg string) uint16 {
	switch arg {
	case "f0", "f1", "f2", "f3":
		return uint16(arg[1]-'0') << 6
	}
	panic("invalid float accumulator")
}

func parseReg(arg string) RegNum {
	switch arg {
	case "r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7":
		return RegNum(arg[1] - '0')
	case "sp":
		return SP
	case "pc":
		return PC
	}
	panic("invalid register")
}

func parseConst(arg string) uint16 {
	if n, err := strconv.ParseUint(arg, 8, 16); err == nil {
		return uint16(n)
	}
	if n, err := strconv.ParseInt(arg, 8, 16); err == nil {
		return uint16(n)
	}
	panic(fmt.Sprintf("invalid constant %q", arg))
}

func parseArg(pc uint16, arg string, shift uint, fp bool, codes []uint16) []uint16 {
	if arg == "" {
		panic("empty arg")
	}
	if !fp && (arg[0] == 'r' || arg[0] == 'p' || arg[0] == 's') {
		r := parseReg(arg)
		codes[0] |= uint16(r) << shift
		return codes
	}
	if fp && len(arg) == 2 && arg[0] == 'f' && '0' <= arg[1] && arg[1] <= '5' {
		codes[0] |= uint16(arg[1]-'0') << shift
		return codes
	}

	mode := uint16(0)
	if arg[0] == '@' {
		mode |= 0o10 // indirect bit
		arg = arg[1:]
		if arg == "" {
			panic("invalid indirect")
		}
	}

	if '0' <= arg[0] && arg[0] <= '7' && !strings.Contains(arg, "(") {
		// pc-relative address, offset loaded from instruction stream
		n := parseConst(arg)
		codes[0] |= (0o67 | mode) << shift
		next := pc + 2*uint16(1+len(codes))
		return append(codes, n-next)
	}
	if arg[0] == '#' {
		// constant loaded from instruction stream
		codes[0] |= (0o27 | mode) << shift
		return append(codes, parseConst(arg[1:]))
	}

	imm, haveImm := uint16(0), false
	if '0' <= arg[0] && arg[0] <= '7' {
		// immediate offset
		i := strings.Index(arg, "(")
		imm, arg = parseConst(arg[:i]), arg[i:]
		haveImm = true
	}
	if arg[0] == '-' {
		if haveImm {
			panic("decrement with immediate")
		}
		mode |= 0o40 // pre-decrement
		arg = arg[1:]
		if arg == "" {
			panic("bad argument syntax")
		}
	}
	if arg[0] != '(' {
		panic("bad argument syntax")
	}
	reg, arg, ok := strings.Cut(arg[1:], ")")
	if !ok {
		panic("bad argument syntax")
	}
	r := parseReg(reg)
	if arg == "+" {
		if haveImm {
			panic("increment with immediate")
		}
		mode |= 0o20 // post-increment
	} else {
		if arg != "" {
			panic("bad argument syntax")
		}
		if haveImm {
			mode |= 0o60
			codes = append(codes, imm)
		}
		if mode == 0 {
			mode = 0o10
		}
	}
	codes[0] |= (mode | uint16(r)) << shift
	return codes
}
