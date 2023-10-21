// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unix

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"rsc.io/unix/pdp11"
)

//go:embed disk.txtar
var FS []byte

type Proc struct {
	procState

	Sys     *System
	CPU     pdp11.CPU      // cpu state
	Mem     pdp11.ArrayMem // process memory
	Args    [4]uint16      // syscall args
	Error   Errno          // syscall error
	Gid     int8           // effective group id
	RUid    int8           // real user id
	RGid    int8           // real group id
	Sig     int8           // pending signal
	Dir     *inode         // directory
	Files   [NOFILE]*File  // fd table
	Signals [NSIG]uint16   // signal handlers
	Prof    [4]uint16
	Times
	Nice      int16
	TextSize  uint16
	DataStart uint16
	DataSize  uint16
	wkey      any
	sched     chan bool
	TTY       *TTY
}

type procState struct {
	status int8
	flag   uint8
	pri    int8   /* priority, negative is high */
	sig    int8   /* signal number sent to this process */
	Uid    int8   /* user id, used to direct tty signals */
	time   int8   /* resident time for scheduling */
	cpu    int8   /* cpu usage for scheduling */
	nice   int8   /* nice for scheduling */
	ttyp   int16  /* controlling tty */
	Pid    int16  /* unique process id */
	Ppid   int16  /* process id of parent */
	addr   uint16 /* address of swappable image */
	size   int16  /* size of swappable image (*64 bytes) */
	wchan  int16  /* event process is awaiting */
	textp  uint16 /* pointer to text structure */
}

type System struct {
	Big      sync.Mutex
	Exit1    sync.Cond
	Disk     *Disk
	Procs    []*Proc
	NextPid  int16
	curpri   int8
	runrun   int8
	swtchpos int
	Timer    time.Time
	TTYRead  uint16     // 1<<X bit means ttyX has a pending read
	TTY      [1 + 8]TTY // TTY[1]..TTY[8] is /dev/tty1..tty8

	idle  chan bool
	Trace bool
}

func (s *System) lookpid(pid int16) *Proc {
	for _, p := range s.Procs {
		if p.Pid == pid {
			return p
		}
	}
	return nil
}

type ProcState int

const (
	ProcNew ProcState = iota
	ProcReady
	ProcRunning
	ProcTrap
	ProcExited
	ProcSleep
	ProcIO
	ProcWait
)

func (ps ProcState) String() string {
	switch ps {
	case ProcNew:
		return "New"
	case ProcReady:
		return "Ready"
	case ProcRunning:
		return "Running"
	case ProcTrap:
		return "Trap"
	case ProcExited:
		return "Exited"
	case ProcWait:
		return "Wait"
	case ProcIO:
		return "IO"
	}
	return fmt.Sprintf("ProcState(%d)", ps)
}

func (p *Proc) str(addr uint16) string {
	b := p.Mem[addr:]
	b, _, ok := bytes.Cut(b, []byte("\x00"))
	if !ok {
		p.Error = EFAULT
		return ""
	}
	return string(b)
}

func (p *Proc) mem(addr, count uint16) []byte {
	if int(addr)+int(count) >= 1<<16 {
		p.Error = EFAULT
		return nil
	}
	return p.Mem[addr : addr+count]
}

type Times struct {
	UTime  int16
	STime  int16
	CUTime [2]int16
	CSTime [2]int16
}

type File struct {
	flag   int
	count  int
	offset int
	inode  *inode
	pipe   *pipe
}

type Signal struct {
}

func (sys *System) Fork(parent *Proc) (*Proc, error) {
	if len(sys.Procs) >= NPROC {
		return nil, fmt.Errorf("too many procs")
	}

	p := sys.newProc()
	p.CPU.R = parent.CPU.R
	p.CPU.PS = parent.CPU.PS
	p.Mem = parent.Mem
	p.Ppid = parent.Pid
	p.Uid = parent.Uid
	p.RUid = p.Uid
	p.Gid = parent.Gid
	p.RGid = p.Gid
	p.Dir = parent.Dir
	p.Files = parent.Files
	p.Signals = parent.Signals
	p.TTY = parent.TTY
	p.ttyp = parent.ttyp
	for _, f := range p.Files {
		if f != nil {
			f.count++
		}
	}
	sys.Procs = append(sys.Procs, p)

	return p, nil
}

func (sys *System) newProc() *Proc {
	p := new(Proc)
	p.Sys = sys
	p.CPU.Mem = &p.Mem
	p.status = _SIDL

Retry:
	pid := sys.NextPid
	if sys.NextPid <= 0 {
		sys.NextPid = 1
		goto Retry
	}
	sys.NextPid++
	for _, op := range sys.Procs {
		if op.Pid == pid {
			goto Retry
		}
	}

	p.Pid = pid
	p.sched = make(chan bool)
	go sys.run(p)
	return p
}

type rw struct {
	io.Reader
	io.Writer
}

func NewSystem(archive []byte) (*System, error) {
	sys := new(System)
	d, err := newDisk(archive)
	if err != nil {
		return nil, err
	}
	sys.Disk = d
	sys.idle = make(chan bool)
	for i := range sys.TTY {
		sys.TTY[i].Sys = sys
	}
	return sys, nil
}

func (sys *System) ReadFile(name string) ([]byte, error) {
	p := &Proc{Sys: sys}
	p.Pid = 1
	p.Ppid = 0
	p.Dir = p.iget(1)
	defer p.iput(p.Dir)

	ip, _, _ := p.namei(name, nameFind)
	if ip == nil {
		return nil, p.Error
	}
	defer p.iput(ip)
	return ip.data, nil
}

func (sys *System) Start(exe []byte, argv []string, stdout io.Writer) (*Proc, error) {
	p := sys.newProc()
	p.Pid = 1
	p.Ppid = 0
	p.Dir = p.iget(1)
	sys.Exit1.L = &sys.Big
	sys.TTY[8].Print = func(b []byte, echo bool) (int, Errno) {
		n, err := stdout.Write(b)
		if err != nil {
			return 0, EIO
		}
		return n, 0
	}
	for i := range sys.TTY {
		sys.TTY[i].major = 4
		sys.TTY[i].minor = uint8(i)
	}

	p.exec(exe, argv, nil)
	if p.Error != 0 {
		return nil, fmt.Errorf("exec: %v", p.Error)
	}

	sys.Procs = append(sys.Procs, p)
	return p, nil
}

func (sys *System) Wait() {
	if !sys.Timer.IsZero() && !time.Now().Before(sys.Timer) {
		sys.Timer = time.Time{}
		sys.wakeup(&sys.Timer)
	}
	// Every proc is waiting on p.sched in p.swtch; waking up any of them is fine
	// since their scheduler loop will find the right next process to run.
	sys.Procs[0].sched <- true
	<-sys.idle
}

func sysfork(p *Proc) {
	c, err := p.Sys.Fork(p)
	if err != nil {
		p.Error = EIO
		return
	}
	p.CPU.R[0] = uint16(c.Pid)
	c.CPU.R[0] = uint16(p.Pid)
	p.CPU.R[pdp11.PC] += 2
	if p.Sys.Trace {
		fmt.Fprintf(os.Stderr, "[pid %d] fork -> %d\n", p.Pid, c.Pid)
	}
	p.Sys.setrun(c)
}

func (sys *System) run(p *Proc) {
	<-p.sched
	if p.status == _SZOMB {
		runtime.Goexit()
	}
	for {
		if p.issig() {
			p.psig()
		}
		pc := p.CPU.R[pdp11.PC]
		n := 100
		if p.Sys.Trace {
			text, next, err := p.CPU.Disasm(pc)
			if err != nil {
				text = "???"
			}
			op, _, _ := strings.Cut(text, " ")
			inst, _ := p.CPU.ReadW(pc)
			fmt.Fprintf(os.Stderr, "%06o %06o %4s %06o %06o %06o %06o %06o %06o %06o   NZVC1 %04b\n", p.CPU.R[pdp11.PC], inst, strings.ToLower(op),
				p.CPU.R[0], p.CPU.R[1], p.CPU.R[2], p.CPU.R[3], p.CPU.R[4], p.CPU.R[5], p.CPU.R[6], p.CPU.PS)
			fmt.Fprintf(os.Stderr, "f0=%v f1=%v f2=%v f3=%v f4=%v f5=%v fps=%v\n",
				p.CPU.F[0], p.CPU.F[1], p.CPU.F[2], p.CPU.F[3], p.CPU.F[4], p.CPU.F[5], p.CPU.FPS)
			fmt.Fprintf(os.Stderr, "# %06o %v (nextPC=%06o)\n", pc, text, next)
			n = 1
		}
		err := p.CPU.Step(n)
		var sig int
		switch err {
		case pdp11.ErrTrap:
			err = Trap(p)
			if p.Error < 100 {
				continue
			}
			sig = SIGSYS
		case pdp11.ErrInst:
			sig = SIGINS
		case pdp11.ErrBPT:
			sig = SIGTRC
		case pdp11.ErrIOT:
			sig = SIGIOT
		case pdp11.ErrEMT:
			sig = SIGEMT
		case pdp11.ErrFPT:
			sig = SIGFPT
		case pdp11.ErrMem:
			sig = SIGSEG
			// TODO stack growth
		}
		if sig != 0 {
			sys.psignal(p, sig)
			continue
		}
		if err != nil {
			log.Fatalf("%06o: %v", p.CPU.R[pdp11.PC], err)
		}
	}
}
