// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/sysent.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

package v6unix

var sysent [64]sysentry

type sysentry struct {
	args uint16
	name string
	impl func(*Proc)
}

func init() {
	sysent = [64]sysentry{
		{0, "null", sysnull},                  /*  0 = indir */
		{0, "exit(%r)", sysexit},              /*  1 = exit */
		{0, "fork() = %d", sysfork},           /*  2 = fork */
		{2, "read(%r, %p, %d) = %q", sysread}, /*  3 = read */
		{2, "write(%r, %q) = %d", syswrite},   /*  4 = write */
		{2, "open(%s, %d) = %d", sysopen},     /*  5 = open */
		{0, "close(%r)", sysclose},            /*  6 = close */
		{0, "wait() = %d, %p", syswait},       /*  7 = wait */
		{2, "create(%s, %p) = %d", syscreate}, /*  8 = create */
		{2, "link(%s, %s)", syslink},          /*  9 = link */
		{1, "unlink(%s)", sysunlink},          /* 10 = unlink */
		{2, "exec(%s, %S)", sysexec},          /* 11 = exec */
		{1, "chdir(%s)", syschdir},            /* 12 = chdir */
		{0, "time() = %d, %d", systime},       /* 13 = time */
		{3, "mknod(", sysmknod},               /* 14 = mknod */
		{2, "chmod(%s, %p)", syschmod},        /* 15 = chmod */
		{2, "chown(%s, %p)", syschown},        /* 16 = chown */
		{1, "break(%p)", sysbreak},            /* 17 = break */
		{2, "stat(%s, %p)", sysstat},          /* 18 = stat */
		{2, "seek(%r, %d, %d) = %d", sysseek}, /* 19 = seek */
		{0, "getpid() = %d", sysgetpid},       /* 20 = getpid */
		{3, "mount()", sysmount},              /* 21 = mount */
		{1, "umount()", sysumount},            /* 22 = umount */
		{0, "setuid(%r)", syssetuid},          /* 23 = setuid */
		{0, "getuid() = %d", sysgetuid},       /* 24 = getuid */
		{0, "stime(%r, %r)", sysstime},        /* 25 = stime */
		{3, "ptrace()", sysptrace},            /* 26 = ptrace */
		{0, "none", sysnone},                  /* 27 = x */
		{1, "fstat(%d, %p)", sysfstat},        /* 28 = fstat */
		{0, "29", sysnone},                    /* 29 = x */
		{1, "smdate", sysnull},                /* 30 = smdate; inoperative */
		{1, "stty(%r, %p)", sysstty},          /* 31 = stty */
		{1, "gtty(%r, %p)", sysgtty},          /* 32 = gtty */
		{0, "33", sysnone},                    /* 33 = x */
		{0, "nice(%r)", sysnice},              /* 34 = nice */
		{0, "sleep(%r)", syssleep},            /* 35 = sleep */
		{0, "sync()", syssync},                /* 36 = sync */
		{1, "kill(%r, %a)", syskill},          /* 37 = kill */
		{0, "csw()", syscsw},                  /* 38 = csw (switch) */
		{0, "39", sysnone},                    /* 39 = x */
		{0, "40", sysnone},                    /* 40 = x */
		{0, "dup(%r) = %d", sysdup},           /* 41 = dup */
		{0, "pipe() = %d, %d", syspipe},       /* 42 = pipe */
		{1, "times", systimes},                /* 43 = times */
		{4, "prof", sysprof},                  /* 44 = prof */
		{0, "45", sysnone},                    /* 45 = tiu */
		{0, "setgid(%r)", syssetgid},          /* 46 = setgid */
		{0, "getgid(%r)", sysgetgid},          /* 47 = getgid */
		{2, "sig(%d, %p)", syssig},            /* 48 = sig */
		{0, "49", sysnone},                    /* 49 = x */
		{0, "50", sysnone},                    /* 50 = x */
		{0, "51", sysnone},                    /* 51 = x */
		{0, "52", sysnone},                    /* 52 = x */
		{0, "53", sysnone},                    /* 53 = x */
		{0, "54", sysnone},                    /* 54 = x */
		{0, "55", sysnone},                    /* 55 = x */
		{0, "56", sysnone},                    /* 56 = x */
		{0, "57", sysnone},                    /* 57 = x */
		{0, "58", sysnone},                    /* 58 = x */
		{0, "59", sysnone},                    /* 59 = x */
		{0, "60", sysnone},                    /* 60 = x */
		{0, "61", sysnone},                    /* 61 = x */
		{0, "62", sysnone},                    /* 62 = x */
		{0, "63", sysnone},                    /* 63 = x */
	}
}
