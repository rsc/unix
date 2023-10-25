// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Analogous to _fs/usr/sys/ken/rdwri.c but the code is new
// since there is no on-disk form, nor inode locking.

package v6unix

func (p *Proc) readi(ip *inode, b []byte, off int) int {
	ip.atime = now()
	if ip.major != 0 {
		return p.dev(ip.major).read(p, ip.minor, b, off)
	}
	if off < 0 || off >= len(ip.data) {
		return 0
	}
	return copy(b, ip.data[off:])
}

func (p *Proc) writei(ip *inode, b []byte, off int) int {
	const maxFileSize = 1<<24 - 1

	ip.atime = now()
	ip.mtime = ip.atime
	if ip.major != 0 {
		return p.dev(ip.major).write(p, ip.minor, b, off)
	}
	if off < 0 || off+len(b) > maxFileSize {
		p.Error = EIO
		return 0
	}
	if len(b) == 0 {
		return 0
	}
	if off+len(b) > len(ip.data) {
		old := len(ip.data)
		new := off + len(b)
		for cap(ip.data) < new {
			ip.data = append(ip.data[:cap(ip.data)], 0)
		}
		clear(ip.data[old:off])
		ip.data = ip.data[:new]
		ip.writeSize()
	}
	ip.mtime = now()
	return copy(ip.data[off:], b)
}
