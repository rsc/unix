// Copyright 2023 The Go Authors and Caldera International Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/fio.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

package unix

/*
 * Convert a user supplied
 * file descriptor into a pointer
 * to a file structure.
 * Only task is to check range
 * of the descriptor.
 */
func (p *Proc) getf(fd uint16) *File {
	if int(fd) >= len(p.Files) || p.Files[fd] == nil {
		p.Error = EBADF
		return nil
	}
	return p.Files[fd]
}

/*
 * Internal form of close.
 * Decrement reference count on
 * file structure and call closei
 * on last closef.
 * Also make sure the pipe protocol
 * does not constipate.
 */
func (p *Proc) closef(f *File) {
	// TODO: why close the pipe on first close instead of last?
	if f.flag&_FPIPE != 0 {
		p.closep(f)
	}
	if f.count <= 1 {
		p.closei(f.inode, f.flag&_FWRITE)
	}
	f.count--
}

/*
 * Decrement reference count on an
 * inode due to the removal of a
 * referencing file structure.
 * On the last closei, switchout
 * to the close entry point of special
 * device handler.
 * Note that the handler gets called
 * on every open and only on the last
 * close.
 */
func (p *Proc) closei(ip *inode, rw int) {
	if ip.count <= 1 {
		if ip.major != 0 {
			p.dev(ip.major).close(p, ip.minor)
		}
	}
	p.iput(ip)
}

/*
 * openi called to allow handler
 * of special files to initialize and
 * validate before actual IO.
 * Called on all sorts of opens
 * and also on mount.
 */
func (p *Proc) openi(ip *inode, rw int) {
	if ip.major != 0 {
		p.dev(ip.major).open(p, ip.minor, rw)
	}
}

/*
 * Check mode permission on inode pointer.
 * Mode is READ, WRITE or EXEC.
 * In the case of WRITE, the
 * read-only status of the file
 * system is checked.
 * Also in WRITE, prototype text
 * segments cannot be written.
 * The mode is shifted to select
 * the owner/group/other fields.
 * The super user is granted all
 * permissions except for EXEC where
 * at least one of the EXEC bits must
 * be on.
 */
func (p *Proc) access(ip *inode, mode uint16) bool {
	if mode == _IWRITE {
		// skip EROFS
		// skip ETXTBSY
	}
	if p.Uid == 0 {
		if mode == _IEXEC && ip.mode&0o111 == 0 {
			p.Error = EACCES
			return false
		}
		return true
	}
	if p.Uid != ip.uid {
		mode >>= 3
		if p.Gid != ip.gid {
			mode >>= 3
		}
	}
	if ip.mode&mode == 0 {
		p.Error = EACCES
		return false
	}
	return true
}

/*
 * Look up a pathname and test if
 * the resultant inode is owned by the
 * current user.
 * If not, try for super-user.
 * If permission is granted,
 * return inode pointer.
 */
func (p *Proc) owner(addr uint16) *inode {
	ip, _, _ := p.namei(p.str(addr), 0)
	if ip == nil {
		return nil
	}
	if p.Uid != ip.uid && !p.suser() {
		p.iput(ip)
		return nil
	}
	return ip
}

/*
 * Test if the current user is the
 * super user.
 */
func (p *Proc) suser() bool {
	if p.Uid == 0 {
		return true
	}
	p.Error = EPERM
	return false
}

/*
 * Allocate a user file descriptor.
 */
func (p *Proc) ufalloc() int {
	for i, f := range p.Files {
		if f == nil {
			p.CPU.R[0] = uint16(i)
			return i
		}
	}
	p.Error = EMFILE
	return -1
}

/*
 * Allocate a user file descriptor
 * and a file structure.
 * Initialize the descriptor
 * to point at the file structure.
 *
 * no file -- if there are no available
 * 	file structures.
 */
func (p *Proc) falloc() *File {
	i := p.ufalloc()
	if i < 0 {
		return nil
	}
	// skip ENFILE
	f := new(File)
	f.count = 1
	p.Files[i] = f
	return f
}
