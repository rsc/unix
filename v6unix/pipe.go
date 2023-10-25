// Copyright 2023 The Go Authors and Caldera International Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/pipe.c but also greatly simplified.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

// TODO: The handling of signals and sleeps might not be right.

package v6unix

import "sync"

type pipe struct {
	read  sync.Cond
	write sync.Cond
	n     int
	buf   [4096]byte
}

func syspipe(p *Proc) {
	ip := p.ialloc()
	if ip == nil {
		return
	}
	rf := p.falloc()
	if rf == nil {
		p.iput(ip)
		return
	}
	r := p.CPU.R[0]
	wf := p.falloc()
	if wf == nil {
		p.Files[r] = nil
		p.iput(ip)
		return
	}
	p.CPU.R[1] = p.CPU.R[0]
	p.CPU.R[0] = r

	pip := new(pipe)
	pip.read.L = &p.Sys.Big
	pip.write.L = &p.Sys.Big

	wf.flag = _FWRITE | _FPIPE
	wf.inode = ip
	wf.pipe = pip

	rf.flag = _FREAD | _FPIPE
	rf.inode = ip
	rf.pipe = pip

	ip.count = 2
	ip.atime = now()
	ip.mtime = ip.atime
	ip.mode = _IALLOC
}

func (p *Proc) readp(f *File, b []byte) int {
	for f.pipe.n == 0 && f.inode.count >= 2 {
		f.pipe.read.Wait()
	}
	n := copy(b, f.pipe.buf[:f.pipe.n])
	copy(f.pipe.buf[:0], f.pipe.buf[n:f.pipe.n])
	f.pipe.n -= n
	f.offset += n
	f.pipe.write.Broadcast()
	return n
}

func (p *Proc) writep(f *File, b []byte) int {
	total := 0
	for len(b) > 0 {
		for f.pipe.n == len(f.pipe.buf) && f.inode.count >= 2 {
			f.pipe.write.Wait()
		}
		if f.inode.count < 2 {
			p.Error = EPIPE
			// psignal(p, SIGPIPE)
			return 0
		}
		n := copy(f.pipe.buf[f.pipe.n:], b)
		f.pipe.n += n
		total += n
		b = b[n:]
		f.offset += n
		f.pipe.read.Broadcast()
	}
	return total
}

func (p *Proc) closep(f *File) {
	f.pipe.read.Broadcast()
	f.pipe.write.Broadcast()
}
