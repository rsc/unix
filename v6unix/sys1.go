// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/sys1.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

package v6unix

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"unsafe"

	"rsc.io/unix/pdp11"
)

/*
 * exec system call.
 */
func sysexec(p *Proc) {
	/*
	 * pick up file names
	 * and check various modes
	 * for execute permission
	 */
	name := p.str(p.Args[0])
	ip, _, _ := p.namei(name, nameFind)
	if ip == nil {
		return
	}
	defer p.iput(ip)
	if !p.access(ip, _IEXEC) {
		return
	}
	if ip.mode&_IFMT != 0 || len(ip.data) < 4*2 {
		p.Error = ENOEXEC
		return
	}

	// load arguments
	const maxArgv = 256
	var argv []string
	for addr := p.Args[1]; ; addr += 2 {
		ap, err := p.CPU.ReadW(addr)
		if err != nil {
			p.Error = EFAULT
			return
		}
		if ap == 0 {
			break
		}
		s := p.str(ap)
		if p.Error != 0 {
			return
		}
		argv = append(argv, s)
		if len(argv) > maxArgv {
			p.Error = E2BIG
			return
		}
	}

	p.exec(ip.data, argv, ip)
}

func (p *Proc) exec(aout []byte, argv []string, ip *inode) {

	af, err := ParseAout(bytes.NewBuffer(aout))
	if af == nil || err != Errno(0) {
		p.Error = err
		return
	}

	const round = 0o20000
	tsr := (af.TextSize() + round - 1) &^ (round - 1)

	// lay out new memory image
	var mem pdp11.ArrayMem
	copy(mem[:af.TextSize()], af.Text)
	copy(mem[tsr:tsr+af.DataSize()], af.Data)

	na := (1 + len(argv) + 1) * 2
	for _, s := range argv {
		na += len(s) + 1
	}
	cp := uint16(0)
	for i := len(argv) - 1; i >= 0; i-- {
		cp -= uint16(len(argv[i]) + 1)
	}
	if -cp > 510 {
		p.Error = E2BIG
		return
	}
	ap := cp - cp&1
	ap -= 2
	*(*uint16)(unsafe.Pointer(&mem[ap])) = ^uint16(0)

	cp = 0
	for i := len(argv) - 1; i >= 0; i-- {
		s := argv[i]
		cp -= uint16(len(s) + 1)
		copy(mem[cp:], s)
		ap -= 2
		*(*uint16)(unsafe.Pointer(&mem[ap])) = cp
	}
	ap -= 2
	*(*uint16)(unsafe.Pointer(&mem[ap])) = uint16(len(argv))
	sp := ap

	p.Mem = mem
	if af.hdr.MagicNum == 0o407 {
		p.TextSize = af.hdr.TextSize
		p.DataStart = af.hdr.TextSize
		p.DataSize = af.hdr.DataSize
	} else {
		p.TextSize = af.TextSize()
		p.DataStart = uint16(tsr)
		p.DataSize = af.DataSize()
	}

	// TODO check STRC
	if true && ip != nil {
		if ip.mode&_ISUID != 0 {
			if p.Uid != 0 {
				p.Uid = ip.uid
			}
		}
		if ip.mode&_ISGID != 0 {
			p.Gid = ip.gid
		}
	}

	// clear sigs, regs, and return
	for i := range p.Signals {
		if p.Signals[i] != 1 {
			p.Signals[i] = 0
		}
	}
	clear(p.CPU.R[:])
	p.CPU.R[pdp11.SP] = sp

	if false {
		for i := 0; i < 1<<16; i += 2 {
			v := *(*uint16)(unsafe.Pointer(&p.Mem[i]))
			if v != 0 {
				fmt.Fprintf(os.Stderr, "start *%06o = %06o\n", i, v)
			}
		}
	}
}

func sysexit(p *Proc) {
	p.Args[0] = p.CPU.R[0] << 8
	p.exit()
}

/*
 * Release resources.
 * Save u. area for parent to look at.
 * Enter zombie state.
 * Wake up parent and init processes,
 * and dispose of children.
 */
func (p *Proc) exit() {
	// p.flag &^= _STRC
	for i := range p.Signals {
		p.Signals[i] = 1
	}
	for _, f := range p.Files {
		if f != nil {
			p.closef(f)
		}
	}
	p.iput(p.Dir)
	p.status = _SZOMB

	parent := p.Sys.lookpid(p.Ppid)
	if parent == nil {
		p.Ppid = 1
		parent = p.Sys.lookpid(1)
		if parent == nil {
			panic("exit no init")
		}
	}
	p.Sys.wakeup(p.Sys.Procs[0])
	p.Sys.wakeup(parent)
	for _, q := range p.Sys.Procs {
		if q.Ppid == p.Pid {
			q.Ppid = 1
			if q.status == _SSTOP {
				p.Sys.setrun(q)
			}
		}
	}
	p.swtch()
}

func syswait(p *Proc) {
	for {
		found := 0
		for i, p1 := range p.Sys.Procs {
			if p1.Ppid == p.Pid {
				found++
				if p1.status == _SZOMB {
					p.Sys.Procs = slices.Delete(p.Sys.Procs, i, i+1)
					p.CSTime[0] += p1.CSTime[0]
					p.CSTime[1] += p1.CSTime[1]
					p.CUTime[0] += p1.CUTime[0]
					p.CUTime[1] += p1.CUTime[1]
					p.CPU.R[0] = uint16(p1.Pid)
					p.CPU.R[1] = p1.Args[0] // wait status
					return
				}
				if p1.status == _SSTOP {
					if p1.flag&_SWTED == 0 {
						p.flag |= _SWTED
						p.CPU.R[0] = uint16(p1.Pid)
						p.CPU.R[1] = uint16(p1.sig)<<8 | 0o177
						return
					}
					p1.flag &^= _STRC | _SWTED
					p.Sys.setrun(p1)
				}
			}
		}
		if found == 0 {
			p.Error = ECHILD
			return
		}
		p.sleep(p, 'w', _PWAIT)
	}
}

func sysbreak(p *Proc) {

}
