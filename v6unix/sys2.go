// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/sys2.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

package v6unix

import (
	"path"
	"time"
)

/*
 * read system call
 */
func sysread(p *Proc) {
	p.rdwr(_FREAD)
}

/*
 * write system call
 */
func syswrite(p *Proc) {
	p.rdwr(_FWRITE)
}

/*
 * common code for read and write calls:
 * check permissions, set base, count, and offset,
 * and switch out to readi, writei, or pipe code.
 */
func (p *Proc) rdwr(mode int) {
	f := p.getf(p.CPU.R[0])
	if f == nil {
		return
	}
	if f.flag&mode == 0 {
		p.Error = EBADF
		return
	}
	b := p.mem(p.Args[0], p.Args[1])
	var n int
	if f.flag&_FPIPE != 0 {
		if mode == _FREAD {
			n = p.readp(f, b)
		} else {
			n = p.writep(f, b)
		}
	} else {
		off := f.offset
		if mode == _FREAD {
			n = p.readi(f.inode, b, off)
		} else {
			n = p.writei(f.inode, b, off)
		}
		f.offset += n
	}
	p.CPU.R[0] = uint16(n)
}

/*
 * open system call
 */
func sysopen(p *Proc) {
	p.open(p.str(p.Args[0]), int(p.Args[1]))
}

func (p *Proc) open(name string, omode int) {
	ip, _, _ := p.namei(name, nameFind)
	if ip == nil {
		return
	}
	p.open1(ip, omode+1, 0)
}

/*
 * create system call
 */
func syscreate(p *Proc) {
	name := p.str(p.Args[0])
	ip, dp, off := p.namei(name, nameCreate)
	defer p.iput(dp)
	if ip != nil {
		p.open1(ip, _FWRITE, 1)
		return
	}
	if p.Error != 0 {
		return
	}
	ip = p.maknode(path.Base(name), (p.Args[1]&0o7777)&^_ISVTX, dp, off)
	if ip == nil {
		return
	}
	p.open1(ip, _FWRITE, 2)
}

/*
 * common code for open and creat.
 * Check permissions, allocate an open file structure,
 * and call the device open routine if any.
 */
func (p *Proc) open1(ip *inode, mode, trf int) {
	if trf != 2 {
		if mode&_FREAD != 0 {
			p.access(ip, _IREAD)
		}
		if mode&_FWRITE != 0 {
			p.access(ip, _IWRITE)
			if ip.mode&_IFMT == _IFDIR {
				p.Error = EISDIR
			}
		}
	}
	if p.Error != 0 {
		p.iput(ip)
		return
	}
	if trf != 0 {
		p.itrunc(ip)
	}

	f := p.falloc()
	if f == nil {
		p.iput(ip)
		return
	}
	f.flag = mode & (_FREAD | _FWRITE)
	f.inode = ip
	fd := p.CPU.R[0]
	p.openi(ip, mode&_FWRITE)
	if p.Error == 0 {
		return
	}
	p.Files[fd] = nil
	p.iput(ip)
}

/*
 * close system call
 */
func sysclose(p *Proc) {
	f := p.getf(p.CPU.R[0])
	if f == nil {
		return
	}
	p.Files[p.CPU.R[0]] = nil
	p.closef(f)
}

/*
 * seek system call
 */
func sysseek(p *Proc) {
	f := p.getf(p.CPU.R[0])
	if f == nil {
		return
	}
	if f.flag&_FPIPE != 0 {
		p.Error = ESPIPE
		return
	}
	ptr := int(p.Args[1])
	off := int(p.Args[0])
	if ptr != 0 && ptr != 3 {
		off = int(int16(p.Args[0]))
	}
	if ptr >= 3 {
		off *= 512
	}
	switch ptr {
	case 0, 3:
		// nothing
	case 1, 4:
		off += f.offset
	default:
		off += f.inode.size()
	}
	f.offset = off
}

/*
 * link system call
 */
func syslink(p *Proc) {
	ip, _, _ := p.namei(p.str(p.Args[0]), nameFind)
	if ip == nil {
		return
	}
	defer p.iput(ip)

	if ip.nlink >= 127 {
		p.Error = EMLINK
		return
	}
	if ip.mode&_IFMT == _IFDIR && !p.suser() {
		return
	}

	name := p.str(p.Args[1])
	xp, dp, off := p.namei(name, nameCreate)
	defer p.iput(dp)
	if xp != nil {
		p.Error = EEXIST
		p.iput(xp)
		return
	}
	if p.Error != 0 {
		return
	}
	// skip EXDEV
	p.wdir(ip, path.Base(name), dp, off)
	ip.nlink++
	ip.mtime = now()
}

/*
 * mknod system call
 */
func sysmknod(p *Proc) {
	if !p.suser() {
		return
	}

	name := p.str(p.Args[0])
	ip, dp, off := p.namei(name, nameCreate)
	defer p.iput(dp)
	if ip != nil {
		p.Error = EEXIST
		p.iput(ip)
		return
	}

	ip = p.maknode(path.Base(name), p.Args[1], dp, off)
	if ip == nil {
		return
	}
	ip.addr[0] = p.Args[2]
	p.iput(ip)
}

/*
 * sleep system call
 * not to be confused with the sleep internal routine.
 */
func syssleep(p *Proc) {
	end := time.Now().Add(time.Duration(p.CPU.R[0]) * time.Second)
	for time.Now().Before(end) {
		if p.Sys.Timer.IsZero() || p.Sys.Timer.After(end) {
			p.Sys.Timer = end
		}
		p.sleep(&p.Sys.Timer, 't', PSLEP)
	}
}
