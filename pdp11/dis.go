// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import (
	"fmt"
	"strings"
)

func (cpu *CPU) Disasm(pc uint16) (asm string, next uint16, err error) {
	code, err := cpu.ReadW(pc)
	if err != nil {
		return "", pc, err
	}
	inst := lookup(code)
	if inst.text == "" {
		return "", pc, fmt.Errorf("unknown instruction %06o", code)
	}
	next = pc + 2
	op, args := parseAsm(inst.text)
	var out []byte
	out = append(out, op...)
	for i, arg := range args {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, ' ')

		switch arg {
		default:
			asm += arg
		case "%b": // branch offset
			out = fmt.Appendf(out, "%o", next+2*uint16(int8(code)))
		case "%B": // sob offset
			out = fmt.Appendf(out, "%o", next-2*(code&077))
		case "%n": // emt/trap number
			out = fmt.Appendf(out, "%o", code&0377)
		case "%N": // spl level
			out = fmt.Appendf(out, "%o", code&07)
		case "%r": // register number at bit 6
			out = fmt.Appendf(out, "%s", RegNum((code>>6)&07))
		case "%R": // register number at bit 0
			out = fmt.Appendf(out, "%s", RegNum((code & 07)))
		case "%d", "%s": // dst, src
			w := code
			if arg == "%s" {
				w >>= 6
			}
			var err error
			if out, next, err = fmtArg(out, cpu, w, next); err != nil {
				return "", pc, err
			}
		case "%a": // fp accumulator
			out = fmt.Appendf(out, "f%d", (code>>6)&0o3)
		case "%f": // fsrc/fdst
			if code&0o70 == 0 {
				if code&0o07 >= 6 {
					return "", pc, fmt.Errorf("unknown instruction %06o", code)
				}
				out = fmt.Appendf(out, "f%d", code&0o07)
			} else {
				if out, next, err = fmtArg(out, cpu, code, next); err != nil {
					return "", pc, err
				}
			}
		}
	}
	return string(out), next, nil
}

func parseAsm(text string) (op string, args []string) {
	op, argstr := strings.TrimSpace(text), ""
	if i := strings.IndexAny(text, " \t"); i >= 0 {
		op, argstr = text[:i], strings.TrimSpace(text[i:])
	}
	args = strings.Split(argstr, ",")
	for i, arg := range args {
		args[i] = strings.TrimSpace(arg)
	}
	for len(args) > 0 && args[len(args)-1] == "" {
		args = args[:len(args)-1]
	}
	return op, args
}

func fmtArg(out []byte, cpu *CPU, w, next uint16) (out1 []byte, next1 uint16, err error) {
	r := RegNum(w & 07)
	mode := (w >> 3) & 07

	// Conveniences for PC-relative data and immediates.
	if r == PC {
		if imm, err := cpu.ReadW(next); err == nil {
			switch mode {
			case 2:
				return fmt.Appendf(out, "#%o", int16(imm)), next + 2, nil
			case 3:
				return fmt.Appendf(out, "@#%o", imm), next + 2, nil
			case 6:
				return fmt.Appendf(out, "%o", next+2+imm), next + 2, nil
			case 7:
				return fmt.Appendf(out, "@%o", next+2+imm), next + 2, nil
			}
		}
	}

	reg := r.String()
	if mode == 0 { // register
		return append(out, reg...), next, nil
	}
	reg = "(" + reg + ")"
	if mode == 1 { // indirect register
		return append(out, reg...), next, nil
	}

	// General memory access.
	indir := ""
	if mode&1 != 0 { // extra indirect
		indir = "@"
	}

	switch mode &^ 1 {
	case 2: // post-increment
		return fmt.Appendf(out, "%s%s+", indir, reg), next, nil
	case 4: // pre-increment
		return fmt.Appendf(out, "%s-%s", indir, reg), next, nil
	case 6: // indexed
		imm, err := cpu.ReadW(next)
		if err != nil {
			return nil, next, err
		}
		next += 2
		return fmt.Appendf(out, "%s%o%s", indir, int16(imm), reg), next, nil
	}
	panic("unreachable")
}
