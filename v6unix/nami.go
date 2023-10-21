// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Analogous to _fs/usr/sys/ken/nami.c but the code is new.
// The original namei is too complex and hard to follow,
// and our in-memory form does not need any of the complications.

package unix

import (
	"unsafe"
)

const (
	nameFind   = 0
	nameCreate = 1
	nameDelete = 2
)

func (p *Proc) namei(name string, op int) (ip, dp *inode, off int) {
	d := p.Sys.Disk
	if name != "" && name[0] == '/' {
		dp = d.inodes[1]
	} else {
		dp = p.Dir
	}
	dp.count++

	// If path is empty, keep reference to start directory.
	if elem, _ := nextElem(name); elem == "" {
		if op == nameFind {
			return dp, nil, 0
		}
		return dp, nil, 0
	}

	// Walk non-empty path.
	for {
		if !p.access(dp, _IEXEC) {
			p.iput(dp)
			return nil, nil, 0
		}
		elem, rest := nextElem(name)
		if elem == "" {
			panic("namei")
		}

		inum, off := dsearch(dp.data, elem)
		if inum == 0 {
			if rest == "" && op == nameCreate && p.access(dp, _IWRITE) {
				dp.mtime = now()
				return nil, dp, off
			}
			if p.Error == 0 {
				p.Error = ENOENT
			}
			p.iput(dp)
			return nil, nil, 0
		}
		ip := p.iget(inum)
		if rest == "" && op == nameDelete {
			if !p.access(dp, _IWRITE) {
				p.iput(ip)
				p.iput(dp)
				return nil, nil, 0
			}
			return ip, dp, off
		}
		p.iput(dp)
		if ip == nil {
			return nil, nil, 0
		}
		if rest == "" {
			return ip, nil, 0
		}
		name = rest
		dp = ip
	}
}

func dsearch(data []byte, elem string) (inum uint16, off int) {
	slot := len(data)
	for i := 0; i < len(data); i += int(direntSize) {
		dir := (*dirent)(unsafe.Pointer(&data[i]))
		if dir.inum != 0 && dir.name() == elem {
			return dir.inum, i
		}
		if slot == len(data) && dir.inum == 0 {
			slot = i
		}
	}
	return 0, slot
}

func nextElem(path string) (elem, rest string) {
	i := 0
	for i < len(path) && path[i] == '/' {
		i++
	}
	path = path[i:]
	if path == "" {
		return "", ""
	}
	i = 0
	for i < len(path) && path[i] != '/' {
		i++
	}
	elem = path[:i]
	for i < len(path) && path[i] == '/' {
		i++
	}
	rest = path[i:]
	if len(elem) > 14 {
		elem = elem[:14]
	}
	return elem, rest
}
