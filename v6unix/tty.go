// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/dmr/tty.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

// TODO: TTY output processing

package unix

import (
	"bytes"
	"unsafe"
)

type TTY struct {
	TDev
	Print func(b []byte, echo bool) (int, Errno)
	State uint16
	Raw   bytes.Buffer // raw input characters
	Canon bytes.Buffer // canonicalized input characters
	EOF   bool
	Sys   *System
	Delct int
}

func (t *TTY) WriteByte(c byte) {
	// Translate modern backspace and ^U to v6 equivalents.
	if c == '\b' || c == 0x7F {
		c = t.erase
	}
	if c == 'U'-'@' {
		c = t.kill
	}

	if c == '\r' && t.flags&CRMOD != 0 {
		c = '\n'
	}
	if t.flags&RAW == 0 && (c == CQUIT || c == CINTR) {
		sig := SIGINT
		if c == CQUIT {
			sig = SIGQIT
		}
		t.Sys.signal(t, sig)
		t.Raw.Truncate(0)
		t.Canon.Truncate(0)
	}
	if t.flags&LCASE != 0 && 'A' <= c && c <= 'Z' {
		c += 'a' - 'A'
	}
	t.Raw.WriteByte(c)
	if t.flags&RAW != 0 || c == '\n' || c == 0o004 {
		t.Raw.WriteByte(0o377)
		t.Delct++
		t.Sys.wakeup(&t.Delct)
	}
	if t.flags&ECHO != 0 && t.Print != nil {
		var buf [1]byte
		buf[0] = c
		t.Print(buf[:], true)
	}
}

type TDev struct {
	_rawq  [3]uint16 /* input chars right off device (not used)*/
	_canq  [3]uint16 /* input chars after erase and kill (not used)*/
	_outq  [3]uint16 /* output list to device (not used) */
	flags  uint16    /* mode, settable by stty call */
	_addr  uint16    /* device address (register or startup fcn; not used) */
	delct  int8      /* number of delimiters in raw q */
	col    int8      /* printing column of device */
	erase  uint8     /* erase character */
	kill   uint8     /* kill character */
	state  uint8     /* internal state, not visible externally */
	_char  uint8     /* character temporary (not used) */
	speeds uint16    /* output+input line speed */
	minor  uint8     /* device name */
	major  uint8
}

/* default special characters */
const (
	CERASE = '#'
	CEOT   = 0o004
	CKILL  = '@'
	CQUIT  = 0o034 /* FS, cntl shift L */
	CINTR  = 0o177 /* DEL */
)

/* modes */
const (
	HUPCL   = 0o1
	XTABS   = 0o2
	LCASE   = 0o4
	ECHO    = 0o10
	CRMOD   = 0o20
	RAW     = 0o40
	ODDP    = 0o100
	EVENP   = 0o200
	NLDELAY = 0o1400
	TBDELAY = 0o6000
	CRDELAY = 0o30000
	VTDELAY = 0o40000
)

/* Hardware bits */
const (
	DONE    = 0o200
	IENABLE = 0o100
)

/* Internal state bits */
const (
	TIMEOUT = 01   /* Delay timeout in progress */
	WOPEN   = 02   /* Waiting for open to complete */
	ISOPEN  = 04   /* Device is open */
	SSTART  = 010  /* Has special start routine at addr */
	CARR_ON = 020  /* Software copy of carrier-present */
	BUSY    = 040  /* Output in progress */
	ASLEEP  = 0100 /* Wakeup when output done */
)

func sysstty(p *Proc) {
	info := (*[3]uint16)(unsafe.Pointer(&p.mem(p.Args[0], 3*2)[0]))
	p.sgtty(p.CPU.R[0], info, nil)
}

func sysgtty(p *Proc) {
	info := (*[3]uint16)(unsafe.Pointer(&p.mem(p.Args[0], 3*2)[0]))
	p.sgtty(p.CPU.R[0], nil, info)
}

func (p *Proc) sgtty(fd uint16, in, out *[3]uint16) {
	f := p.getf(fd)
	if f == nil {
		return
	}
	ip := f.inode
	if ip.mode&_IFMT != _IFCHR {
		p.Error = ENOTTY
		return
	}
	p.dev(ip.major).sgtty(p, ip.minor, in, out)
}

type ttydev struct{}

func (ttydev) open(p *Proc, minor uint8, rw int) {
	if minor > 8 {
		p.Error = ENXIO
	}
	tty := &p.Sys.TTY[minor]
	if tty.State&ISOPEN == 0 {
		tty.state |= ISOPEN | CARR_ON
		tty.flags = XTABS | LCASE | ECHO | CRMOD
		tty.erase = CERASE
		tty.kill = CKILL
	}
	if p.TTY == nil {
		p.TTY = tty
		p.ttyp = memTTY + memTTYSize*int16(minor)
	}
}

func (ttydev) read(p *Proc, minor uint8, b []byte, off int) int {
	if minor > 8 {
		p.Error = ENXIO
	}
	if len(b) == 0 {
		return 0
	}
	tty := &p.Sys.TTY[minor]
	for {
		n, _ := tty.Canon.Read(b)
		if n > 0 {
			return n
		}
		if tty.Delct > 0 {
			tty.canon()
			n, _ = tty.Canon.Read(b)
			return n
		}
		p.Sys.TTYRead |= 1 << minor
		p.sleep(&tty.Delct, 'i', PSLEP)
		p.Sys.TTYRead &^= 1 << minor
	}
}

var maptab = [256]byte{
	0o0, 0o0, 0o0, 0o0, 0o4, 0o0, 0o0, 0o0,
	0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	0o0, '|', 0o0, '#', 0o0, 0o0, 0o0, '`',
	'{', '}', 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	'@', 0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0, 0o0,
	0o0, 0o0, 0o0, 0o0, 0o0, 0o0, '~', 0o0,
	0o0, 'A', 'B', 'C', 'D', 'E', 'F', 'G',
	'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O',
	'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W',
	'X', 'Y', 'Z', 0o0, 0o0, 0o0, 0o0, 0o0,
}

func (t *TTY) canon() {
Loop:
	var canon []byte
	for {
		c, err := t.Raw.ReadByte()
		if err != nil {
			panic("ttycanon") // cannot happen - we know t.Delct is set
		}
		if c == 0o377 {
			t.Delct--
			break
		}
		if t.flags&RAW == 0 {
			cn := len(canon)
			if cn < 1 || canon[cn-1] != '\\' {
				if c == t.erase {
					if cn > 0 {
						canon = canon[:cn-1]
					}
					continue
				}
				if c == t.kill {
					goto Loop
				}
				if c == CEOT {
					continue
				}
			} else if maptab[c] != 0 && (maptab[c] == c || t.flags&LCASE != 0) {
				if cn < 2 || canon[cn-2] != '\\' {
					c = maptab[c]
				}
				cn--
			}
			canon = canon[:cn]
		}
		canon = append(canon, c)
		// if len(canon) >= CANBSIZ { break }
	}
	t.Canon.Write(canon)
}

func (ttydev) write(p *Proc, minor uint8, b []byte, off int) int {
	if minor > 8 {
		p.Error = EIO
		return 0
	}
	tty := &p.Sys.TTY[minor]
	if tty.Print == nil {
		p.Error = EIO
		return 0
	}
	n, errno := tty.Print(b, false)
	if errno != 0 {
		p.Error = errno
	}
	return n
}

func (ttydev) close(p *Proc, minor uint8) {
	if minor > 8 {
		p.Error = EIO
		return
	}
	tty := &p.Sys.TTY[minor]
	tty.State = 0
}

func (ttydev) sgtty(p *Proc, minor uint8, in, out *[3]uint16) {
	if minor > 8 {
		p.Error = EIO
		return
	}
	tty := &p.Sys.TTY[minor]
	if out != nil {
		out[0] = tty.speeds
		out[1] = uint16(tty.erase) | uint16(tty.kill)<<8
		out[2] = tty.flags
	}
	if in != nil {
		tty.speeds = in[0]
		tty.erase = uint8(in[1])
		tty.kill = uint8(in[1] >> 8)
		tty.flags = in[2]
	}
}
