// Copyright 2023 The Go Authors and Caldera International Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unix

import "unsafe"

type inode struct {
	count int
	stat
	data []byte
}

type stat struct {
	dev    uint16
	inum   uint16
	mode   uint16
	nlink  int8
	uid    int8
	gid    int8
	sizeHi uint8
	sizeLo uint16
	minor  uint8 // also addr[0]
	major  uint8
	addr   [7]uint16
	atime  [2]uint16
	mtime  [2]uint16
}

func (s *stat) size() int {
	return int(s.sizeHi<<8) | int(s.sizeLo)
}

func (ip *inode) writeSize() {
	n := len(ip.data)
	if n >= 1<<24 {
		n = 1<<24 - 1
	}
	ip.sizeHi = uint8(n >> 16)
	ip.sizeLo = uint16(n)
}

type dirent struct {
	inum uint16
	nam  [DIRSIZ]byte
}

const direntSize = unsafe.Sizeof(dirent{})

func (d *dirent) bytes() []byte {
	return (*[direntSize]byte)(unsafe.Pointer(d))[:]
}

func (d *dirent) name() string {
	b := d.nam[:]
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			b = b[:i]
		}
	}
	return string(b)
}

/* modes */
const (
	_IALLOC uint16 = 0100000 /* file is used */
	_IFMT   uint16 = 060000  /* type of file */
	_IFDIR  uint16 = 040000  /* directory */
	_IFCHR  uint16 = 020000  /* character special */
	_IFBLK  uint16 = 060000  /* block special, 0 is regular */
	_ILARG  uint16 = 010000  /* large addressing algorithm */
	_ISUID  uint16 = 04000   /* set user id on execution */
	_ISGID  uint16 = 02000   /* set group id on execution */
	_ISVTX  uint16 = 01000   /* save swapped text even after use */
	_IREAD  uint16 = 0400    /* read, write, execute permissions */
	_IWRITE uint16 = 0200
	_IEXEC  uint16 = 0100
)

const (
	_FREAD int = 1 << iota
	_FWRITE
	_FPIPE
)
