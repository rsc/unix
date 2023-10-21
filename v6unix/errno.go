// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unix

import "fmt"

const (
	EPERM Errno = 1 + iota
	ENOENT
	ESRCH
	EINTR
	EIO
	ENXIO
	E2BIG
	ENOEXEC
	EBADF
	ECHILD
	EAGAIN
	ENOMEM
	EACCES
	ENOTBLK
	EBUSY
	EEXIST
	EXDEV
	ENODEV
	ENOTDIR
	EISDIR
	EINVAL
	ENFILE
	EMFILE
	ENOTTY
	ETXTBSY
	EFBIG
	ENOSPC
	ESPIPE
	EROFS
	EMLINK
	EPIPE
	EFAULT Errno = 106
)

type Errno int8

func (e Errno) Error() string {
	if e == EFAULT {
		return "EFAULT"
	}
	if 0 <= e && int(e) < len(enames) && enames[e] != "" {
		return enames[e]
	}
	return fmt.Sprintf("Errno(%d)", int(e))
}

var enames = []string{
	"",
	"EPERM",
	"ENOENT",
	"ESRCH",
	"EINTR",
	"EIO",
	"ENXIO",
	"E2BIG",
	"ENOEXEC",
	"EBADF",
	"ECHILD",
	"EAGAIN",
	"ENOMEM",
	"EACCES",
	"ENOTBLK",
	"EBUSY",
	"EEXIST",
	"EXDEV",
	"ENODEV",
	"ENOTDIR",
	"EISDIR",
	"EINVAL",
	"ENFILE",
	"EMFILE",
	"ENOTTY",
	"ETXTBSY",
	"EFBIG",
	"ENOSPC",
	"ESPIPE",
	"EROFS",
	"EMLINK",
	"EPIPE",
}
