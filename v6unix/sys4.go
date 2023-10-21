// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/sys4.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

package unix

import (
	"time"
	"unsafe"
)

func syscsw(p *Proc) {
	p.CPU.R[0] = 0 // TODO
}

var start = time.Now()

// Latest time stamps in disks are on /dev, at 177300290.
// Make the system boot to that time,
// since date cannot display years like 2023.
const boottime = 177300290

func now() [2]uint16 {
	t := boottime + int64(time.Since(start).Seconds())
	var tm [2]uint16
	tm[0] = uint16(t >> 16)
	tm[1] = uint16(t)
	return tm
}

func systime(p *Proc) {
	t := now()
	p.CPU.R[0] = t[0]
	p.CPU.R[1] = t[1]
}

func sysstime(p *Proc) {
	if p.suser() {
		// TODO set time from p.CPU.R[0] and p.CPU.R[1]
		// TODO wake up any sleepers
	}
}

func syssetuid(p *Proc) {
	uid := int8(p.CPU.R[0])
	if p.RUid == uid || p.suser() {
		p.Uid = uid
		p.RUid = uid
	}
}

func sysgetuid(p *Proc) {
	p.CPU.R[0] = uint16(p.Uid)<<8 | uint16(p.RUid)
}

func syssetgid(p *Proc) {
	gid := int8(p.CPU.R[0])
	if p.RGid == gid || p.suser() {
		p.Gid = gid
		p.RGid = gid
	}
}

func sysgetgid(p *Proc) {
	p.CPU.R[0] = uint16(p.Gid)<<8 | uint16(p.RGid)
}

func sysgetpid(p *Proc) {
	p.CPU.R[0] = uint16(p.Pid)
}

func syssync(p *Proc) {
	// TODO
}

func sysnice(p *Proc) {
	n := int16(p.CPU.R[0])
	if n > 20 {
		n = 20
	}
	if n < 0 && !p.suser() {
		n = 0
	}
	p.Nice = n
}

/*
 * Unlink system call.
 * panic: unlink -- "cannot happen"
 */
func sysunlink(p *Proc) {
	p.unlink(p.str(p.Args[0]))
}

func (p *Proc) unlink(name string) {
	ip, dp, off := p.namei(name, nameDelete)
	if ip == nil {
		return
	}
	defer p.iput(ip)
	defer p.iput(dp)

	if ip.mode&_IFMT == _IFDIR && !p.suser() {
		return
	}

	clear(dp.data[off : off+DIRSIZ+2])
	ip.nlink--
	ip.mtime = now()
}

func syschdir(p *Proc) {
	ip, _, _ := p.namei(p.str(p.Args[0]), 0)
	if ip == nil {
		return
	}
	if ip.mode&_IFMT != _IFDIR {
		p.Error = ENOTDIR
		p.iput(ip)
		return
	}
	if !p.access(ip, _IEXEC) {
		p.iput(ip)
		return
	}
	p.iput(p.Dir)
	p.Dir = ip
}

func syschmod(p *Proc) {
	ip := p.owner(p.Args[0])
	if ip == nil {
		return
	}
	ip.mode &^= 0o7777
	if p.Uid != 0 {
		p.Args[1] &^= _ISVTX
	}
	ip.mode |= p.Args[1] & 0o7777
	ip.mtime = now()
	p.iput(ip)
}

func syschown(p *Proc) {
	if !p.suser() {
		return
	}
	ip := p.owner(p.Args[0])
	if ip == nil {
		return
	}
	ip.uid = int8(p.Args[1])
	ip.gid = int8(p.Args[1] >> 8)
	ip.mtime = now()
	p.iput(ip)
}

/*
 * Change modified date of file:
 * time to r0-r1; sys smdate; file
 * This call has been withdrawn because it messes up
 * incremental dumps (pseudo-old files aren't dumped).
 * It works though and you can uncomment it if you like.
func syssmdate(p *Proc) {
	ip := p.owner(p.Args[0])
	if ip == nil {
		return
	}
	ip.mtime = [2]uint16{p.CPU.R[0], p.CPU.R[1]}
	p.iput(ip)
}
*/

func syssig(p *Proc) {
	a := p.Args[0]
	if a >= NSIG || a == SIGKIL {
		p.Error = EINVAL
		return
	}
	p.CPU.R[0] = p.Signals[a]
	p.Signals[a] = p.Args[1]
	if p.Sig == int8(a) {
		p.Sig = 0
	}
}

func syskill(p *Proc) {
	p.kill(int16(p.CPU.R[0]), int(p.Args[0]))
}

func (p *Proc) kill(pid int16, sig int) {
	found := 0
	for _, p1 := range p.Sys.Procs {
		if p1 == p {
			continue
		}
		if pid != 0 && p1.Pid != pid {
			continue
		}
		if pid == 0 && (p1.TTY != p.TTY || p1.Pid == 1) {
			continue
		}
		if p.Uid != 0 && p1.Uid != p.Uid {
			continue
		}
		found++
		p.Sys.psignal(p1, sig)
	}
	if found == 0 {
		p.Error = ESRCH
	}
}

func systimes(p *Proc) {
	addr := p.Args[0]
	for i, t := range (*[6]uint16)(unsafe.Pointer(&p.UTime)) {
		if err := p.CPU.WriteW(addr+2*uint16(i), t); err != nil {
			p.Error = EFAULT
		}
	}
}

func sysprof(p *Proc) {
	p.Prof[0] = p.Args[0] &^ 1 // base of sample buf
	p.Prof[1] = p.Args[1]      // size of same
	p.Prof[2] = p.Args[2]      // pc offset
	p.Prof[3] = p.Args[3] >> 1 // pc scale
}
