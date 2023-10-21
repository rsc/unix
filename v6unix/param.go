// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/param.h and _fs/usr/sys/proc.h.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

package unix

/*
 * tunable variables
 */
const (
	NBUF = 15 /* size of buffer cache */
	// NINODE  = 100       /* number of in core inodes */
	// NFILE   = 100       /* number of in core file structures */
	// NMOUNT  = 5         /* number of mountable file systems */
	// NEXEC   = 3         /* number of simultaneous exec's */
	MAXMEM  = (64 * 32) /* max core per process - first # is Kw */
	SSIZE   = 20        /* initial stack size (*64 bytes) */
	SINCR   = 20        /* increment of stack (*64 bytes) */
	NOFILE  = 15        /* max open files per process */
	CANBSIZ = 256       /* max size of typewriter line */
	CMAPSIZ = 100       /* size of core allocation area */
	SMAPSIZ = 100       /* size of swap allocation area */
	NCALL   = 20        /* max simultaneous time callouts */
	NPROC   = 50        /* max number of processes */
	NTEXT   = 40        /* max number of pure texts */
	NCLIST  = 100       /* max total clist size */
	HZ      = 60        /* Ticks/second of the clock */
)

/*
 * priorities
 * probably should not be
 * altered too much
 */
const (
	PSWP   = -100
	PINOD  = -90
	PRIBIO = -50
	PPIPE  = 1
	PWAIT  = 40
	PSLEP  = 90
	PUSER  = 100
)

/*
 * signals
 * dont change
 */
const (
	NSIG    = 20
	SIGHUP  = 1  /* hangup */
	SIGINT  = 2  /* interrupt (rubout) */
	SIGQIT  = 3  /* quit (FS) */
	SIGINS  = 4  /* illegal instruction */
	SIGTRC  = 5  /* trace or breakpoint */
	SIGIOT  = 6  /* iot */
	SIGEMT  = 7  /* emt */
	SIGFPT  = 8  /* floating exception */
	SIGKIL  = 9  /* kill */
	SIGBUS  = 10 /* bus error */
	SIGSEG  = 11 /* segmentation violation */
	SIGSYS  = 12 /* sys */
	SIGPIPE = 13 /* end of pipe */
)

/*
 * fundamental constants
 * cannot be changed
 */
const (
	USIZE   = 16 /* size of user block (*64) */
	NULL    = 0
	NODEV   = (-1)
	ROOTINO = 1  /* i number of all roots */
	DIRSIZ  = 14 /* max characters per directory */
)

const (
	/* status codes */
	_SSLEEP int8 = 1 /* sleeping on high priority */
	_SWAIT  int8 = 2 /* sleeping on low priority */
	_SRUN   int8 = 3 /* running */
	_SIDL   int8 = 4 /* intermediate state in process creation */
	_SZOMB  int8 = 5 /* intermediate state in process termination */
	_SSTOP  int8 = 6 /* process being traced */

	/* flag codes */
	_SLOAD uint8 = 01  /* in core */
	_SSYS  uint8 = 02  /* scheduling process */
	_SLOCK uint8 = 04  /* process cannot be swapped */
	_SSWAP uint8 = 010 /* process is being swapped out */
	_STRC  uint8 = 020 /* process is being traced */
	_SWTED uint8 = 040 /* another tracing flag */

	/* priorities */
	_PSWP   int8 = -100
	_PINOD  int8 = -90
	_PRIBIO int8 = -50
	_PPIPE  int8 = 1
	_PWAIT  int8 = 40
	_PSLEP  int8 = 90
	_PUSER  int8 = 100
)
