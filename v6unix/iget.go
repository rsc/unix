// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Analogous to _fs/usr/sys/ken/iget.c but the code is new
// since there is no on-disk form, nor inode locking.

package v6unix

func (p *Proc) iget(inum uint16) *inode {
	d := p.Sys.Disk
	if int(inum) >= len(d.inodes) || d.inodes[inum] == nil {
		p.Error = EIO
		return nil
	}
	ip := d.inodes[inum]
	ip.count++
	return ip
}

func (p *Proc) iput(ip *inode) {
	if ip == nil {
		return
	}
	d := p.Sys.Disk
	ip.count--
	if ip.count == 0 {
		if ip.nlink == 0 {
			d.inodes[ip.inum] = nil
			return
		}
	}
}

func (p *Proc) itrunc(ip *inode) {
	if ip.mode&(_IFCHR|_IFBLK) != 0 {
		return
	}
	ip.data = nil
	ip.writeSize()
	ip.mtime = now()
}

func (p *Proc) maknode(name string, mode uint16, dp *inode, off int) *inode {
	ip := p.ialloc()
	if ip == nil {
		return nil
	}
	ip.atime = now()
	ip.mtime = ip.atime
	ip.mode = mode | _IALLOC
	ip.nlink = 1
	ip.uid = p.Uid
	ip.gid = p.Gid
	p.wdir(ip, name, dp, off)
	return ip
}

func (p *Proc) wdir(ip *inode, name string, dp *inode, off int) {
	var de dirent
	de.inum = ip.inum
	copy(de.nam[:], name)
	if off == len(dp.data) {
		dp.data = append(dp.data, de.bytes()...)
		dp.writeSize()
	} else {
		copy(dp.data[off:], de.bytes())
	}
}
