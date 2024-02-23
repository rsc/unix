// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v6unix

import (
	"encoding/binary"
	"io"
)

// https://man.cat-v.org/unix-6th/5/a.out

// Aout file is assembler and link editor output of unix v6
type Aout struct {
	hdr  AoutHdr
	Text []byte
	Data []byte
	BSS  []byte
}

// AoutHdr header of a.out file
type AoutHdr struct {
	/*
	   1  A magic number (407, 410, or 411(8))
	   2  The size of the program text segment
	   3  The size of the initialized portion of the data segment
	   4  The size of the uninitialized (bss) portion of the data
	      segment
	   5  The size of the symbol table
	   6  The entry location (always 0 at present)
	   7  Unused
	   8  A flag indicating relocation bits have been suppressed
	*/
	MagicNum     uint16
	TextSize     uint16
	DataSize     uint16
	BSSSize      uint16
	SymTableSize uint16
	Entry        uint16
	_            uint16 // Unused
	RelocFlag    uint16
}

const maxTextSize = 50000

func ParseAout(rdr io.Reader) (*Aout, Errno) {
	af := &Aout{}
	err := binary.Read(rdr, binary.LittleEndian, &af.hdr)
	if err != nil {
		return nil, ENOEXEC
	}
	switch af.hdr.MagicNum {
	default:
		return nil, ENOEXEC
	case 0o407, 0o410, 0o411:
	}

	if (af.TextSize()|af.DataSize())&1 != 0 {
		return nil, ENOEXEC
	}

	if af.TextSize()+af.DataSize() > maxTextSize {
		return nil, E2BIG
	}

	af.Text = make([]byte, af.TextSize())
	n, err := rdr.Read(af.Text)
	if uint16(n) != af.TextSize() || err != nil {
		return nil, ENOEXEC
	}

	af.Data = make([]byte, af.DataSize())
	n, err = rdr.Read(af.Data)
	if uint16(n) != af.DataSize() || err != nil {
		return nil, ENOEXEC
	}

	return af, 0
}

func (a *Aout) DataSize() uint16 {
	switch a.hdr.MagicNum {
	case 0o407:
		return a.hdr.TextSize + a.hdr.DataSize
	case 0o410, 0o411:
		return a.hdr.DataSize
	}
	panic("invalid aout")
}

func (a *Aout) TextSize() uint16 {
	switch a.hdr.MagicNum {
	case 0o407:
		return 0
	case 0o410, 0o411:
		return a.hdr.TextSize
	}
	panic("invalid aout")
}
