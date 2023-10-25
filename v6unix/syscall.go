// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v6unix

import (
	"fmt"
	"os"
	"strings"

	"rsc.io/unix/pdp11"
)

func Trap(p *Proc) error {
	if p.Sys.Trace {
		fmt.Fprintf(os.Stderr, "[pid %d] TRAP\n", p.Pid)
	}
	trap := p.CPU.Inst & 0o77
	p.CPU.R[pdp11.PC] += 2
	argp := p.CPU.R[pdp11.PC]
	otrap := trap
	if trap == 0 {
		p.CPU.R[pdp11.PC] += 2 // consume argp
		var err error
		// old := argp
		argp, err = p.CPU.ReadW(argp)
		if err != nil {
			return err
		}
		// fmt.Fprintf(os.Stderr, "argp *%06o = %06o\n", old, argp)
		// old = argp
		trap, err = p.CPU.ReadW(argp)
		if err != nil {
			return err
		}
		// fmt.Fprintf(os.Stderr, "trap *%06o = %06o\n", old, trap)
		argp += 2
		if trap&^0o77 != 0o104400 {
			return fmt.Errorf("invalid indirect trap %06o", trap)
		}
		trap &= 0o77
	}
	if int(trap) >= len(sysent) {
		return fmt.Errorf("invalid syscall %#o", trap)
	}
	old := argp
	sys := &sysent[trap]
	for i := 0; i < int(sys.args); i++ {
		var err error
		p.Args[i], err = p.CPU.ReadW(argp)
		if err != nil {
			return err
		}
		argp += 2
	}
	if otrap != 0 {
		p.CPU.R[pdp11.PC] = argp
	}

	var desc []byte
	if p.Sys.Trace {
		reg := 0
		arg := 0
		for i := 0; i < len(sys.name); i++ {
			if c := sys.name[i]; c != '%' {
				desc = append(desc, c)
				if c == ')' {
					break
				}
				continue
			}
			i++
			switch c := sys.name[i]; c {
			case 'r':
				desc = fmt.Appendf(desc, "%d", int16(p.CPU.R[reg]))
				reg++
			case 's':
				desc = fmt.Appendf(desc, "%q", p.str(p.Args[arg]))
				arg++
			case 'p', 'S':
				desc = fmt.Appendf(desc, "%06o", p.Args[arg])
				arg++
			case 'd':
				desc = fmt.Appendf(desc, "%d", int16(p.Args[arg]))
				arg++
			case 'q':
				desc = fmt.Appendf(desc, "%q", p.mem(p.Args[arg], p.Args[arg+1]))
				arg += 2
			default:
				desc = append(desc, '%', c)
			}
		}

		fmt.Fprintf(os.Stderr, "[pid %d] trap %06o %s %06o %06o\n", p.Pid, old, desc, p.CPU.R[:], p.Args[:sys.args])
	}

	p.Error = 0
	interrupted := false
	func() {
		defer func() {
			if e := recover(); e != nil {
				fmt.Fprintf(os.Stderr, "[pid %d] trap INTR %06o %s %06o %06o\n", p.Pid, old, desc, p.CPU.R[:], p.Args[:sys.args])
				if e == "sleep interrupted" {
					interrupted = true
					return
				}
				panic(e)
			}
		}()
		sys.impl(p)
	}()
	if p.Sys.Trace {
		fmt.Fprintf(os.Stderr, "[pid %d] trap DONE %06o %s %06o %06o\n", p.Pid, old, desc, p.CPU.R[:], p.Args[:sys.args])
	}
	if interrupted {
		p.Error = EINTR
	}
	p.CPU.PS.SetC(false)
	if p.Error != 0 {
		p.CPU.PS.SetC(true)
		p.CPU.R[0] = uint16(p.Error)
	}

	if p.Sys.Trace {
		if p.Error != 0 {
			desc = fmt.Appendf(desc, ": %v", p.Error)
		} else if i := strings.Index(sys.name, ")"); i >= 0 {
			reg := 0
			for i++; i < len(sys.name); i++ {
				if c := sys.name[i]; c != '%' {
					desc = append(desc, c)
					continue
				}
				i++
				switch c := sys.name[i]; c {
				case 'd':
					desc = fmt.Appendf(desc, "%d", int16(p.CPU.R[reg]))
					reg++
				case 'p':
					desc = fmt.Appendf(desc, "%06o", p.CPU.R[reg])
					reg++
				case 'q':
					desc = fmt.Appendf(desc, "%q", p.mem(p.Args[0], p.Args[1]))
				default:
					desc = append(desc, '%', c)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "[pid %d] %s\n", p.Pid, desc)
	}

	return nil
}

func sysnull(p *Proc) {
}

func sysnone(p *Proc) {
	p.Error = 100
}
