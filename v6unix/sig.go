// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/sig.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

package unix

import "rsc.io/unix/pdp11"

/*
 * Send the specified signal to
 * all processes with 'tp' as its
 * controlling teletype.
 * Called by tty.c for quits and
 * interrupts.
 */
func (sys *System) signal(tty *TTY, sig int) {
	for _, p := range sys.Procs {
		if p.TTY == tty {
			sys.psignal(p, sig)
		}
	}
}

/*
 * Send the specified signal to
 * the specified process.
 */
func (sys *System) psignal(p *Proc, sig int) {
	if sig >= NSIG {
		return
	}
	if p.sig != SIGKIL {
		p.sig = int8(sig)
	}
	/*BUG: should be pri
	if p.stat > PUSER {
		p.stat = PUSER
	}
	*/
	if p.status == _SWAIT {
		sys.setrun(p)
	}
}

/*
 * Returns true if the current
 * process has a signal to process.
 * This is asked at least once
 * each time a process enters the
 * system.
 * A signal does not do anything
 * directly to a process; it sets
 * a flag that asks the process to
 * do something to itself.
 */
func (p *Proc) issig() bool {
	if n := p.sig; n != 0 {
		if p.flag&_STRC != 0 {
			p.stop()
			if n = p.sig; n == 0 {
				return false
			}
		}
		if p.Signals[n]&1 == 0 {
			return true
		}
	}
	return false
}

/*
 * Enter the tracing STOP state.
 * In this state, the parent is
 * informed and the process is able to
 * receive commands from the parent.
 */
func (p *Proc) stop() {
Loop:
	if p.Ppid != 1 {
		for _, p1 := range p.Sys.Procs {
			if p1.Pid == p.Ppid {
				p.Sys.wakeup(p1)
				p.status = _SSTOP
				p.swtch()
				if p.flag&_STRC == 0 || p.procxmt() {
					return
				}
				goto Loop
			}
		}
	}
	p.exit()
}

/*
 * Perform the action specified by
 * the current signal.
 * The usual sequence is:
 *	if p.issig() {
 *		p.psig()
 *	}
 */
func (p *Proc) psig() {
	sig := p.sig
	p.sig = sig
	if pc := p.Signals[sig]; pc != 0 {
		p.Error = 0
		if sig != SIGINS && sig != SIGTRC {
			p.Signals[sig] = 0
		}
		sp := p.CPU.R[pdp11.SP] - 4
		p.grow(sp)
		p.Mem.WriteW(sp+2, uint16(p.CPU.PS))
		p.Mem.WriteW(sp, uint16(p.CPU.R[pdp11.PC]))
		p.CPU.R[pdp11.SP] = sp
		// TODO p.CPU.PS &^= _TBIT
		p.CPU.R[pdp11.PC] = pc
		return
	}

	switch sig {
	case SIGQIT,
		SIGINS,
		SIGTRC,
		SIGIOT,
		SIGEMT,
		SIGFPT,
		SIGBUS,
		SIGSEG,
		SIGSYS:
		p.Args[0] = uint16(sig)
		if p.core() {
			p.Args[0] |= 0o200
		}
	}
	// TODO: This overwrites the p.Args[0] set in the switch
	p.Args[0] = p.CPU.R[0]<<8 | uint16(sig)
	p.exit()
}

/*
 * Create a core image on the file "core"
 * If you are looking for protection glitches,
 * there are probably a wealth of them here
 * when this occurs to a suid command.
 *
 * It writes USIZE block of the
 * user.h area followed by the entire
 * data+stack segments.
 */
func (p *Proc) core() bool {
	return false
	/*
		register s, *ip;
		extern schar;

		u.u_error = 0;
		u.u_dirp = "core";
		ip = namei(&schar, 1);
		if(ip == NULL) {
			if(u.u_error)
				return(0);
			ip = maknode(0666);
			if(ip == NULL)
				return(0);
		}
		if(!access(ip, IWRITE) &&
		   (ip->i_mode&IFMT) == 0 &&
		   u.u_uid == u.u_ruid) {
			itrunc(ip);
			u.u_offset[0] = 0;
			u.u_offset[1] = 0;
			u.u_base = &u;
			u.u_count = USIZE*64;
			u.u_segflg = 1;
			writei(ip);
			s = u.u_procp->p_size - USIZE;
			estabur(0, s, 0, 0);
			u.u_base = 0;
			u.u_count = s*64;
			u.u_segflg = 0;
			writei(ip);
		}
		iput(ip);
		return(u.u_error==0);
	*/
}

/*
 * grow the stack to include the SP
 * true return if successful.
 */
func (p *Proc) grow(sp uint16) bool {
	return true
	/*
		register a, si, i;

		if(sp >= -u.u_ssize*64)
			return(0);
		si = ldiv(-sp, 64) - u.u_ssize + SINCR;
		if(si <= 0)
			return(0);
		if(estabur(u.u_tsize, u.u_dsize, u.u_ssize+si, u.u_sep))
			return(0);
		expand(u.u_procp->p_size+si);
		a = u.u_procp->p_addr + u.u_procp->p_size;
		for(i=u.u_ssize; i; i--) {
			a--;
			copyseg(a-si, a);
		}
		for(i=si; i; i--)
			clearseg(--a);
		u.u_ssize =+ si;
		return(1);
	*/
}

/*
 * sys-trace system call.
 */
func sysptrace(p *Proc) {
	return
	/*
			if p.Args[2] <= 0 {
				p.flag |= _STRC
				return
			}

			for _, p1 := range p.Sys.Procs {
				if
			for (p=proc; p < &proc[NPROC]; p++)
				if (p->p_stat==SSTOP
				 && p->p_pid==u.u_arg[0]
				 && p->p_ppid==u.u_procp->p_pid)
					goto found;
			u.u_error = ESRCH;
			return;

		    found:
			while (ipc.ip_lock)
				sleep(&ipc, IPCPRI);
			ipc.ip_lock = p->p_pid;
			ipc.ip_data = u.u_ar0[R0];
			ipc.ip_addr = u.u_arg[1] & ~01;
			ipc.ip_req = u.u_arg[2];
			p->p_flag =& ~SWTED;
			setrun(p);
			while (ipc.ip_req > 0)
				sleep(&ipc, IPCPRI);
			u.u_ar0[R0] = ipc.ip_data;
			if (ipc.ip_req < 0)
				u.u_error = EIO;
			ipc.ip_lock = 0;
			wakeup(&ipc);
	*/
}

/*
 * Code that the child process
 * executes to implement the command
 * of the parent process in tracing.
 */
func (p *Proc) procxmt() bool {
	return false
	/*
		register int i;
		register int *p;

		if (ipc.ip_lock != u.u_procp->p_pid)
			return(0);
		i = ipc.ip_req;
		ipc.ip_req = 0;
		wakeup(&ipc);
		switch (i) {

		/* read user I * /
		case 1:
			if (fuibyte(ipc.ip_addr) == -1)
				goto error;
			ipc.ip_data = fuiword(ipc.ip_addr);
			break;

		/* read user D * /
		case 2:
			if (fubyte(ipc.ip_addr) == -1)
				goto error;
			ipc.ip_data = fuword(ipc.ip_addr);
			break;

		/* read u * /
		case 3:
			i = ipc.ip_addr;
			if (i<0 || i >= (USIZE<<6))
				goto error;
			ipc.ip_data = u.inta[i>>1];
			break;

		/* write user I (for now, always an error) * /
		case 4:
			if (suiword(ipc.ip_addr, 0) < 0)
				goto error;
			suiword(ipc.ip_addr, ipc.ip_data);
			break;

		/* write user D * /
		case 5:
			if (suword(ipc.ip_addr, 0) < 0)
				goto error;
			suword(ipc.ip_addr, ipc.ip_data);
			break;

		/* write u * /
		case 6:
			p = &u.inta[ipc.ip_addr>>1];
			if (p >= u.u_fsav && p < &u.u_fsav[25])
				goto ok;
			for (i=0; i<9; i++)
				if (p == &u.u_ar0[regloc[i]])
					goto ok;
			goto error;
		ok:
			if (p == &u.u_ar0[RPS]) {
				ipc.ip_data =| 0170000;	/* assure user space * /
				ipc.ip_data =& ~0340;	/* priority 0 * /
			}
			*p = ipc.ip_data;
			break;

		/* set signal and continue * /
		case 7:
			u.u_procp->p_sig = ipc.ip_data;
			return(1);

		/* force exit * /
		case 8:
			exit();

		default:
		error:
			ipc.ip_req = -1;
		}
		return(0);
	*/
}
