// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// V6disk converts a Research Unix Sixth Edition disk image to the
// txtar disk format used by the v6unix package and related commands.
//
// Usage:
//
//	v6disk [-o out.txtar] [-r root] [-x] diskfile
//
// The -r flag specifies the name of the root inode on the disk (default /).
//
// The -o flag specifies the name of the output file to write (default standard output).
//
// The -x flag inverts the operation: diskfile is now a txtar disk, and -o is the
// name of a directory to write the files into (default _fs).
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/tools/txtar"
)

var (
	outfile  = flag.String("o", "", "write output txtar to `file` (default standard output)")
	rootname = flag.String("r", "/", "use `name` for the root inode")
	xflag    = flag.Bool("x", false, "extract txtar disk")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: v6disk [-o out.txtar] [-r root] [-x] diskfile\n")
	os.Exit(2)
}

type filsys struct {
	isize  uint16      /* size in blocks of I list */
	fsize  uint16      /* size in blocks of entire volume */
	nfree  uint16      /* number of in core free blocks (0-100) */
	free   [100]uint16 /* in core free blocks */
	ninode uint16      /* number of in core I nodes (0-100) */
	inode  [100]uint16 /* in core free I nodes */
	flock  uint8       /* lock during free list manipulation */
	ilock  uint8       /* lock during I list manipulation */
	fmod   uint8       /* super block modified flag */
	ronly  uint8       /* mounted read-only flag */
	time   [2]uint16   /* current date of last update */
	pad    [50]uint16
}

type dirent struct {
	ino uint16
	nam [14]byte
}

func (de *dirent) name() string {
	for i := 0; i < 14; i++ {
		if de.nam[i] == 0 {
			return string(de.nam[:i])
		}
	}
	return string(de.nam[:])
}

type inode struct {
	mode   uint16
	nlink  int8
	uid    int8
	gid    int8
	sizeHi uint8
	sizeLo uint16
	addr   [8]uint16
	atime  utime
	mtime  utime
}

type utime struct {
	hi uint16
	lo uint16
}

func (t utime) unix() int64 {
	return int64(t.hi)<<16 | int64(t.lo)
}

func (ip *inode) size() int {
	return int(ip.sizeHi)<<16 | int(ip.sizeLo)
}

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

type iname struct {
	parent *iname
	name   string
	*inode
	content []byte
}

func (n *iname) path() string {
	if n.parent == nil {
		return *rootname
	}
	return path.Join(n.parent.path(), n.name)
}

func main() {
	log.SetPrefix("v6disk: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		usage()
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		log.Fatal(err)
	}

	if *xflag {
		if *outfile == "" {
			*outfile = "_fs"
		}
		ar := txtar.Parse(data)
		for _, f := range ar.Files {
			name, _, _ := strings.Cut(f.Name, " ")
			targ := path.Join(*outfile, name)
			if strings.Contains(f.Name, "mode=0140") || strings.Contains(f.Name, "mode=015") {
				if err := os.MkdirAll(targ, 0777); err != nil {
					log.Fatal(err)
				}
			} else if strings.Contains(f.Name, "mode=0100") || strings.Contains(f.Name, "mode=011") {
				data := f.Data
				if strings.Contains(f.Name, "base64=1") {
					dec, err := base64.StdEncoding.DecodeString(string(data))
					if err != nil {
						log.Fatalf("decoding %s: %v", name, err)
					}
					data = dec
				}
				if err := os.WriteFile(targ, data, 0666); err != nil {
					log.Fatal(err)
				}
			}
		}
		return
	}

	if len(data) < 512 {
		log.Fatalf("disk too small")
	}

	fsys := (*filsys)(unsafe.Pointer(&data[0]))
	if int(fsys.fsize)*512 != len(data) {
		log.Fatalf("corrupt disk: invalid size: %d != %d", int(fsys.fsize)*512, len(data))
	}
	if 2*512+int(fsys.isize)*512 > len(data) {
		log.Fatalf("corrupt disk: too many inodes")
	}

	inodes := unsafe.Slice((*inode)(unsafe.Pointer(&data[2*512])), int(fsys.isize)*512/32)
	inames := make([]iname, len(inodes))
	var list []*iname
	for i := range inodes {
		ip := &inodes[i]
		if ip.mode == 0 {
			continue
		}
		var content []byte
		if m := ip.mode & _IFMT; m != _IFCHR && m != _IFBLK {
			size := ip.size()
			content = make([]byte, size)
			for j := 0; j < size; j += 512 {
				copy(content[j:], getblk(data, ip, j/512))
			}
		}
		inam := &inames[i]
		list = append(list, inam)
		inam.content = content
		inam.inode = ip
		if ip.mode&_IFMT == _IFDIR {
			if len(content) == 0 {
				fmt.Printf("#%d NO CONTENT\n", i+1)
				continue
			}
			dirs := unsafe.Slice((*dirent)(unsafe.Pointer(&content[0])), len(content)/16)
			for j := range dirs {
				de := &dirs[j]
				name := de.name()
				if de.ino == 0 || name == "." || name == ".." {
					continue
				}
				cnam := &inames[de.ino-1]
				cnam.parent = inam
				cnam.name = name
			}
		}
	}

	w := os.Stdout
	if *outfile != "" {
		f, err := os.Create(*outfile)
		if err != nil {
			log.Fatal(err)
		}
		w = f
	}

	slices.SortFunc(list, func(x, y *iname) int { return pathCompare(x.path(), y.path()) })

	for _, inam := range list {
		dev := ""
		if m := inam.mode & _IFMT; m == _IFCHR || m == _IFBLK {
			dev = fmt.Sprintf(" major=%d minor=%d", uint8(inam.addr[0]>>8), uint8(inam.addr[0]))
		}
		b64 := ""
		var c []byte
		if inam.mode&_IFMT == 0 {
			c = inam.content
			if !utf8.Valid(c) || bytes.HasPrefix(c, []byte("-- ")) || bytes.Contains(c, []byte("\n-- ")) || !bytes.HasSuffix(c, []byte("\n")) {
				// base64 encode
				b64 = " base64=1"
				c = []byte(wrap(base64.StdEncoding.EncodeToString(c)))
			}
		}

		fmt.Fprintf(w, "-- %s mode=%07o uid=%d gid=%d atime=%d mtime=%d%s%s --\n%s",
			inam.path(), inam.mode, inam.uid, inam.gid, inam.atime.unix(), inam.mtime.unix(), dev, b64, c)
	}
}

func wrap(text string) string {
	if len(text) < 70 {
		return text + "\n"
	}
	return text[:70] + "\n" + wrap(text[70:])
}

func pathCompare(x, y string) int {
	return strings.Compare(strings.ReplaceAll(x, "/", "\x01"), strings.ReplaceAll(y, "/", "\x01"))
}

func getblk(disk []byte, ip *inode, bn int) []byte {
	if bn&^0o77777 != 0 {
		panic("block too big")
	}

	var addr uint16
	if ip.mode&_ILARG == 0 {
		addr = ip.addr[bn]
	} else {
		i := bn >> 8
		if i > 7 {
			i = 7
		}
		addr = ip.addr[i]
		if addr == 0 {
			// zero addresses could be handled by using a block of zeros,
			// but more likely it is a problem. If we find a disk that needs it,
			// we can add support.
			log.Fatalf("corrupt disk: zero address")
		}
		ptrs := (*[256]uint16)(diskblk(disk, addr))
		if i == 7 {
			addr = ptrs[(bn>>8)&0o377-7]
			if addr == 0 {
				log.Fatalf("corrupt disk: zero address")
			}
			ptrs = (*[256]uint16)(diskblk(disk, addr))
		}
		addr = ptrs[bn&0o377]
		if addr == 0 {
			log.Fatalf("corrupt disk: zero address")
		}
	}
	return (*[512]byte)(diskblk(disk, addr))[:]
}

func diskblk(disk []byte, addr uint16) unsafe.Pointer {
	off := int(addr) * 512
	if off < 0 || off+512 > len(disk) {
		log.Fatalf("corrupt disk: address out of bounds")
	}
	return unsafe.Pointer(&disk[off])
}
