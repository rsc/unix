// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/sys3.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

package v6unix

import (
	"unsafe"
)

/*
 * the fstat system call.
 */
func sysfstat(p *Proc) {
	p.fstat(p.CPU.R[0], (*stat)(unsafe.Pointer(&p.mem(p.Args[0], uint16(unsafe.Sizeof(stat{})))[0])))
}

func (p *Proc) fstat(fd uint16, st *stat) {
	f := p.getf(p.CPU.R[0])
	if f == nil {
		return
	}
	*st = f.inode.stat
}

/*
 * the stat system call.
 */
func sysstat(p *Proc) {
	p.stat(p.str(p.Args[0]), (*stat)(unsafe.Pointer(&p.mem(p.Args[1], uint16(unsafe.Sizeof(stat{})))[0])))
}

func (p *Proc) stat(name string, st *stat) {
	ip, _, _ := p.namei(name, 0)
	if ip == nil {
		return
	}
	*st = ip.stat
	p.iput(ip)
}

/*
 * the dup system call.
 */
func sysdup(p *Proc) {
	f := p.getf(p.CPU.R[0])
	if f == nil {
		return
	}
	i := p.ufalloc()
	if i < 0 {
		return
	}
	p.Files[i] = f
	f.count++
}

/*
 * the mount system call.
 */
func sysmount(p *Proc) {
	p.Error = EINVAL
}

/*
 * the umount system call.
 */
func sysumount(p *Proc) {
	p.Error = EINVAL
}
