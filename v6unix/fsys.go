// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v6unix

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"golang.org/x/tools/txtar"
)

const maxInodes = 1 << 15

type Disk struct {
	inodes []*inode
}

func (p *Proc) ialloc() *inode {
	d := p.Sys.Disk
	for {
		for i, ip := range d.inodes {
			if i > 0 && ip == nil {
				ip := new(inode)
				ip.inum = uint16(i)
				ip.count = 1
				p.Sys.Disk.inodes[i] = ip
				return ip
			}
		}
		if len(d.inodes) >= maxInodes {
			p.Error = ENOSPC
			return nil
		}
		d.inodes = append(d.inodes, nil)
	}
}

func newDisk(archive []byte) (*Disk, error) {
	d := new(Disk)
	d.inodes = []*inode{nil, {stat: stat{inum: 1, nlink: 1, mode: _IALLOC | _IFDIR | 0o555}}}

	var p Proc // root user identity
	p.Sys = &System{Disk: d}
	root := d.inodes[1]
	p.wdir(root, ".", root, 0)
	p.wdir(root, "..", root, DIRSIZ+2)

	ar := txtar.Parse(archive)
	for _, file := range ar.Files {
		f := strings.Fields(file.Name)
		var st stat
		name := f[0]
		link := ""
		b64 := false
		for _, arg := range f[1:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				return nil, fmt.Errorf("invalid txtar k=v: %s", arg)
			}
			if k == "link" {
				link = v
				continue
			}
			i, err := strconv.ParseInt(v, 0, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid txtar k=v: %s", arg)
			}
			switch k {
			default:
				return nil, fmt.Errorf("invalid txtar k=v: %s", arg)
			case "mode":
				st.mode = uint16(i)
			case "uid":
				st.uid = int8(i)
			case "gid":
				st.gid = int8(i)
			case "major":
				st.major = uint8(i)
			case "minor":
				st.minor = uint8(i)
			case "atime":
				st.atime[0] = uint16(i >> 16)
				st.atime[1] = uint16(i)
			case "mtime":
				st.mtime[0] = uint16(i >> 16)
				st.mtime[1] = uint16(i)
			case "base64":
				b64 = i != 0
			}
		}

		ip, dp, off := p.namei(name, nameCreate)
		if ip == nil && dp == nil {
			return nil, fmt.Errorf("%v: %v", name, p.Error)
		}
		if link != "" {
			lp, _, _ := p.namei(link, nameFind)
			if lp == nil {
				p.iput(ip)
				p.iput(dp)
				return nil, fmt.Errorf("%v: %v", link, p.Error)
			}
			p.wdir(lp, path.Base(name), dp, off)
		} else {
			if ip == nil {
				ip = p.maknode(path.Base(name), st.mode, dp, off)
				if ip == nil {
					p.iput(dp)
					return nil, fmt.Errorf("%v: %v", name, p.Error)
				}
				if ip.mode&_IFMT == _IFDIR {
					ip.data = make([]byte, 2*(DIRSIZ+2))
					p.wdir(ip, ".", ip, 0)
					p.wdir(dp, "..", ip, DIRSIZ+2)
				}
			}
			st.dev = ip.dev
			st.inum = ip.inum
			st.nlink = ip.nlink
			ip.stat = st
			if ip.mode&_IFMT == 0 {
				if b64 {
					dec, err := base64.StdEncoding.DecodeString(string(file.Data))
					if err != nil {
						return nil, fmt.Errorf("%s: decoding: %v", name, err)
					}
					ip.data = dec
				} else {
					ip.data = file.Data
				}
			}
			ip.writeSize()
		}
		p.iput(ip)
		p.iput(dp)

		if p.Error != 0 {
			return nil, fmt.Errorf("%v: %v", name, p.Error)
		}
	}

	return d, nil
}

func (d *Disk) sync(file string) error {
	// TODO build inode name list
	// TODO loop over inodes and names creating archive
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(d); err != nil {
		return err
	}
	return os.WriteFile(file, buf.Bytes(), 0666)
}
