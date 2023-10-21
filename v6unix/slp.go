// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported from _fs/usr/sys/ken/sig.c.
//
// Copyright 2001-2002 Caldera International Inc. All rights reserved.
// Use of this source code is governed by a 4-clause BSD-style
// license that can be found in the LICENSE file.

// TODO interrupts and panics

package unix

import "runtime"

/*
 * Give up the processor till a wakeup occurs
 * on chan, at which time the process
 * enters the scheduling queue at priority pri.
 * The most important effect of pri is that when
 * pri<0 a signal cannot disturb the sleep;
 * if pri>=0 signals will be processed.
 * Callers of this routine must be prepared for
 * premature return, and check that the reason for
 * sleeping has gone away.
 */
func (p *Proc) sleep(wkey any, wchan int16, pri int8) {
	if pri >= 0 && p.issig() {
		panic("sleep interrupted")
	}

	p.wkey = wkey
	p.wchan = wchan
	p.status = _SWAIT
	p.swtch()
	if pri >= 0 && p.issig() {
		panic("qsav")
	}
}

/*
 * Wake up all processes sleeping on chan.
 */
func (sys *System) wakeup(wkey any) {
	for _, p := range sys.Procs {
		if p.wkey == wkey {
			sys.setrun(p)
		}
	}
}

/*
 * Set the process running.
 * No swap.
 */
func (sys *System) setrun(p *Proc) {
	if p.status == _SZOMB {
		panic("zombie")
	}
	p.wkey = nil
	p.wchan = 0
	p.status = _SRUN
	if p.pri < sys.curpri {
		sys.runrun++
	}
}

/*
 * Set user priority.
 * The rescheduling flag (runrun)
 * is set if the priority is higher
 * than the currently running process.
 */
func (p *Proc) setpri(p1 *Proc) {
	pri := int16(uint8(p1.cpu) / 16)
	pri += int16(_PUSER) + int16(p1.nice)
	if pri > 127 {
		pri = 127
	}
	if pri > int16(p.Sys.curpri) {
		p.Sys.runrun++
	}
	p1.pri = int8(pri)
}

// Note: There is no sched, because everything is in core.

func (p *Proc) swtch() {
	var next *Proc

	for {
		/*
		 * Search for highest-priority runnable process
		 */
		i := p.Sys.swtchpos
		for j := range p.Sys.Procs {
			i := (i + j) % len(p.Sys.Procs)
			p1 := p.Sys.Procs[i]
			if p1.status == _SRUN && (next == nil || p.pri < next.pri) {
				next = p1
				p.Sys.swtchpos = i
			}
		}

		/*
		 * If no process is runnable, idle.
		 */
		if next != nil {
			if next.sched == nil {
				panic("swtch")
			}
			next.sched <- true
		} else {
			p.Sys.idle <- true
		}
		if p.status == _SZOMB {
			runtime.Goexit()
		}
		<-p.sched
		if p.status == _SRUN {
			break
		}
	}
}
