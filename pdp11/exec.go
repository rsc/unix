// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import (
	"fmt"
	"runtime"
)

func (cpu *CPU) Step(n int) (err error) {
	var old CPU
	defer func() {
		if e := recover(); e != nil {
			*cpu = old
			if _, ok := e.(runtime.Error); ok {
				panic(e)
			}
			if e1, ok := e.(error); ok {
				err = e1
			} else {
				err = fmt.Errorf("%v", e)
			}
		}
	}()

	for ; n > 0; n-- {
		old = *cpu
		pc := cpu.R[PC]
		if pc&1 != 0 {
			panic(ErrInst)
		}
		w, err := cpu.ReadW(pc)
		if err != nil {
			panic(err)
		}
		cpu.Inst = w
		old.Inst = w
		cpu.R[PC] = pc + 2
		lookup(w).do(cpu)
	}
	return nil
}

type addr uint32

const addrReg addr = 1 << 16

func (a addr) String() string {
	if a&addrReg != 0 {
		return RegNum(a).String()
	}
	return fmt.Sprintf("*%06o", a)
}

func (cpu *CPU) addr(enc, size uint16) addr {
	reg := RegNum(enc & 07)
	mode := (enc >> 3) & 07
	if mode == 0 {
		return addrReg | addr(reg)
	}
	if mode&1 != 0 || reg == PC || reg == SP && size == 1 {
		size = 2
	}
	a := cpu.R[reg]
	switch mode &^ 1 {
	case 2:
		// post-increment
		cpu.R[reg] = a + size
	case 4:
		// pre-decrement
		a -= size
		// fmt.Fprintf(os.Stderr, "WB %d %o\n", reg, a)
		cpu.R[reg] = a
	case 6:
		// index offset from PC
		pc := cpu.R[PC]
		imm := cpu.readW(addr(pc))
		cpu.R[PC] = pc + 2
		a = cpu.R[reg] + imm // reload reg in case reg is PC
	}
	if mode != 1 && mode&1 == 1 {
		// extra dereference
		a = cpu.readW(addr(a))
	}
	return addr(a)
}

func (cpu *CPU) readW(a addr) uint16 {
	if a&addrReg != 0 {
		return cpu.R[a&07]
	}
	val, err := cpu.Mem.ReadW(uint16(a))
	if err != nil {
		panic(err)
	}
	// fmt.Fprintf(os.Stderr, "read *%06o = %06o\n", uint16(a), val)
	return val
}

func (cpu *CPU) readB(a addr) uint8 {
	if a&addrReg != 0 {
		return uint8(cpu.R[a&07])
	}
	val, err := cpu.Mem.ReadB(uint16(a))
	if err != nil {
		panic(err)
	}
	// fmt.Fprintf(os.Stderr, "read *%06o = %03o\n", uint16(a), val)
	return val
}

func (cpu *CPU) writeW(a addr, val uint16) {
	if a&addrReg != 0 {
		cpu.R[a&07] = val
		return
	}
	if err := cpu.Mem.WriteW(uint16(a), val); err != nil {
		panic(err)
	}
	// fmt.Fprintf(os.Stderr, "write *%06o = %06o\n", uint16(a), val)
}

func (cpu *CPU) writeB(a addr, val uint8) {
	if a&addrReg != 0 {
		cpu.R[a&07] = cpu.R[a&07]&0o177400 | uint16(val)
		return
	}
	if err := cpu.Mem.WriteB(uint16(a), val); err != nil {
		panic(err)
	}
	// fmt.Fprintf(os.Stderr, "write *%06o = %03o\n", uint16(a), val)
}

func (cpu *CPU) dstAddrB() addr { return cpu.addr(cpu.Inst&077, 1) }
func (cpu *CPU) dstAddrW() addr { return cpu.addr(cpu.Inst&077, 2) }
func (cpu *CPU) dstAddrL() addr { return cpu.addr(cpu.Inst&077, 4) }
func (cpu *CPU) dstAddrF() addr {
	size := uint16(4)
	if cpu.FPS&FD != 0 {
		size = 8
	}
	return cpu.addr(cpu.Inst&077, size)
}
func (cpu *CPU) srcAddrB() addr { return cpu.addr((cpu.Inst>>6)&077, 1) }
func (cpu *CPU) srcAddrW() addr { return cpu.addr((cpu.Inst>>6)&077, 2) }

func (cpu *CPU) dstB() uint8  { return cpu.readB(cpu.dstAddrB()) }
func (cpu *CPU) dstW() uint16 { return cpu.readW(cpu.dstAddrW()) }
func (cpu *CPU) srcB() uint8  { return cpu.readB(cpu.srcAddrB()) }
func (cpu *CPU) srcW() uint16 { return cpu.readW(cpu.srcAddrW()) }

func (cpu *CPU) regArg() RegNum { return RegNum((cpu.Inst >> 6) & 07) }

// unary arithmetic

func xclr(cpu *CPU) {
	dp := cpu.dstAddrW()
	cpu.writeW(dp, 0)
	cpu.PS = PS_Z
}

func xclrb(cpu *CPU) {
	dp := cpu.dstAddrB()
	cpu.writeB(dp, 0)
	cpu.PS = PS_Z
}

func xcom(cpu *CPU) {
	dp := cpu.dstAddrW()
	dst := ^cpu.readW(dp)
	cpu.writeW(dp, dst)
	cpu.PS = PS_C
	cpu.PS.setNZ(dst)
}

func xcomb(cpu *CPU) {
	dp := cpu.dstAddrB()
	dst := ^cpu.readB(dp)
	cpu.writeB(dp, dst)
	cpu.PS = PS_C
	cpu.PS.setNZB(dst)
}

func xdec(cpu *CPU) {
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp) - 1
	cpu.writeW(dp, dst)
	cpu.PS.SetV(dst == 0o077777)
	cpu.PS.setNZ(dst)
}

func xdecb(cpu *CPU) {
	dp := cpu.dstAddrB()
	dst := cpu.readB(dp) - 1
	cpu.writeB(dp, dst)
	cpu.PS.SetV(dst == 0o177)
	cpu.PS.setNZB(dst)
}

func xinc(cpu *CPU) {
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp) + 1
	cpu.writeW(dp, dst)
	cpu.PS.SetV(dst == 0o100000)
	cpu.PS.setNZ(dst)
}

func xincb(cpu *CPU) {
	dp := cpu.dstAddrB()
	dst := cpu.readB(dp) + 1
	cpu.writeB(dp, dst)
	cpu.PS.SetV(dst == 0o200)
	cpu.PS.setNZB(dst)
}

func xneg(cpu *CPU) {
	dp := cpu.dstAddrW()
	dst := -cpu.readW(dp)
	cpu.writeW(dp, dst)
	cpu.PS.SetC(dst != 0)
	cpu.PS.SetV(dst == 0o100000)
	cpu.PS.setNZ(dst)
}

func xnegb(cpu *CPU) {
	dp := cpu.dstAddrB()
	dst := -cpu.readB(dp)
	cpu.writeB(dp, dst)
	cpu.PS.SetC(dst != 0)
	cpu.PS.SetV(dst == 0o200)
	cpu.PS.setNZB(dst)
}

func xtst(cpu *CPU) {
	dst := cpu.readW(cpu.dstAddrW())
	cpu.PS.SetC(false)
	cpu.PS.SetV(false)
	cpu.PS.setNZ(dst)
}

func xtstb(cpu *CPU) {
	dst := cpu.readB(cpu.dstAddrB())
	cpu.PS.SetC(false)
	cpu.PS.SetV(false)
	cpu.PS.setNZB(dst)
}

// shifts and rotates

func setVxor(cpu *CPU) {
	cpu.PS.SetV(cpu.PS.N()^cpu.PS.C() != 0)
}

func xasl(cpu *CPU) {
	dp := cpu.dstAddrW()
	old := cpu.readW(dp)
	dst := old << 1
	cpu.writeW(dp, dst)
	cpu.PS.SetC(old>>15 != 0)
	cpu.PS.setNZ(dst)
	setVxor(cpu)
}

func xaslb(cpu *CPU) {
	dp := cpu.dstAddrB()
	old := cpu.readB(dp)
	dst := old << 1
	cpu.writeB(dp, dst)
	cpu.PS.SetC(old>>7 != 0)
	cpu.PS.setNZB(dst)
	setVxor(cpu)
}

func xasr(cpu *CPU) {
	dp := cpu.dstAddrW()
	old := cpu.readW(dp)
	dst := uint16(int16(old) >> 1)
	cpu.writeW(dp, dst)
	cpu.PS.SetC(old&1 != 0)
	cpu.PS.setNZ(dst)
	setVxor(cpu)
}

func xasrb(cpu *CPU) {
	dp := cpu.dstAddrB()
	old := cpu.readB(dp)
	dst := uint8(int8(old) >> 1)
	cpu.writeB(dp, dst)
	cpu.PS.SetC(old&1 != 0)
	cpu.PS.setNZB(dst)
	setVxor(cpu)
}

func xrol(cpu *CPU) {
	dp := cpu.dstAddrW()
	old := cpu.readW(dp)
	dst := old<<1 | cpu.PS.C()
	cpu.writeW(dp, dst)
	cpu.PS.setNZ(dst)
	cpu.PS.SetC(old>>15 != 0)
	setVxor(cpu)
}

func xrolb(cpu *CPU) {
	dp := cpu.dstAddrB()
	old := cpu.readB(dp)
	dst := old<<1 | uint8(cpu.PS.C())
	cpu.writeB(dp, dst)
	cpu.PS.setNZB(dst)
	cpu.PS.SetC(old>>7 != 0)
	setVxor(cpu)
}

func xror(cpu *CPU) {
	dp := cpu.dstAddrW()
	old := cpu.readW(dp)
	dst := old>>1 | cpu.PS.C()<<15
	cpu.writeW(dp, dst)
	cpu.PS.setNZ(dst)
	cpu.PS.SetC(old&1 != 0)
	setVxor(cpu)
}

func xrorb(cpu *CPU) {
	dp := cpu.dstAddrB()
	old := cpu.readB(dp)
	dst := old>>1 | uint8(cpu.PS.C())<<7
	cpu.writeB(dp, dst)
	cpu.PS.setNZB(dst)
	cpu.PS.SetC(old&1 != 0)
	setVxor(cpu)
}

func xswab(cpu *CPU) {
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp)
	dst = dst<<8 | dst>>8
	cpu.writeW(dp, dst)
	cpu.PS.setNZB(uint8(dst))
	cpu.PS.SetV(false)
	cpu.PS.SetC(false)
}

// multiprecision arithmetic

func xadc(cpu *CPU) {
	dp := cpu.dstAddrW()
	carry := cpu.PS.C()
	dst := cpu.readW(dp) + carry
	cpu.writeW(dp, dst)
	cpu.PS.setNZ(dst)
	cpu.PS.SetV(carry == 1 && dst == 0o100000)
	cpu.PS.SetC(carry == 1 && dst == 0)
}

func xadcb(cpu *CPU) {
	dp := cpu.dstAddrB()
	carry := cpu.PS.C()
	dst := cpu.readB(dp) + uint8(carry)
	cpu.writeB(dp, dst)
	cpu.PS.setNZB(dst)
	cpu.PS.SetV(carry == 1 && dst == 0o200)
	cpu.PS.SetC(carry == 1 && dst == 0)
}

func xsbc(cpu *CPU) {
	dp := cpu.dstAddrW()
	carry := cpu.PS.C()
	dst := cpu.readW(dp) - carry
	cpu.writeW(dp, dst)
	cpu.PS.setNZ(dst)
	cpu.PS.SetV(dst+carry == 0o100000) // manual does not say "carry == 1"; apout agrees
	cpu.PS.SetC(carry == 1 && dst == 0o177777)
}

func xsbcb(cpu *CPU) {
	dp := cpu.dstAddrB()
	carry := cpu.PS.C()
	dst := cpu.readB(dp) - uint8(carry)
	cpu.writeB(dp, dst)
	cpu.PS.setNZB(dst)
	cpu.PS.SetV(dst+uint8(carry) == 0o200)
	cpu.PS.SetC(carry == 1 && dst == 0o377)
}

func xsxt(cpu *CPU) {
	dp := cpu.dstAddrW()
	dst := uint16(int16(cpu.PS.N()<<15) >> 15)
	cpu.writeW(dp, dst)
	cpu.PS.setNZ(dst)
}

// double operand instructions

func xmov(cpu *CPU) {
	src := cpu.srcW()
	dp := cpu.dstAddrW()
	cpu.writeW(dp, src)
	// fmt.Fprintf(os.Stderr, "mov %06o -> %s\n", src, dp)
	cpu.PS.SetV(false)
	cpu.PS.setNZ(src)
}

func xmovb(cpu *CPU) {
	src := cpu.srcB()
	dp := cpu.dstAddrB()
	if dp&addrReg != 0 {
		cpu.writeW(dp, uint16(int8(src))) // sign-extend
	} else {
		cpu.writeB(dp, src)
	}
	cpu.PS.SetV(false)
	cpu.PS.setNZB(src)
}

func xcmp(cpu *CPU) {
	src := cpu.srcW()
	dst := cpu.dstW()
	out := uint32(src) - uint32(dst)
	// fmt.Fprintf(os.Stderr, "cmp %06o %06o %07o %06o\n", src, dst, out, uint16(out))
	cpu.PS.setNZ(uint16(out))
	cpu.PS.SetC(out>>16 != 0)
	cpu.PS.SetV(src>>15 != dst>>15 && dst>>15 == uint16(out)>>15)
}

func xcmpb(cpu *CPU) {
	src := cpu.srcB()
	dst := cpu.dstB()
	out := uint16(src) - uint16(dst)
	// fmt.Fprintf(os.Stderr, "cmpb %03o %03o %04o %06o\n", src, dst, out, uint16(out))
	cpu.PS.setNZ(uint16(out) << 8)
	cpu.PS.SetC(out>>8 != 0)
	cpu.PS.SetV(src>>7 != dst>>7 && dst>>7 == uint8(out)>>7)
}

func xsub(cpu *CPU) {
	src := cpu.srcW()
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp)
	out := uint32(dst) - uint32(src)
	cpu.writeW(dp, uint16(out))
	cpu.PS.setNZ(uint16(out))
	cpu.PS.SetC(out>>16 != 0)
	cpu.PS.SetV(src>>15 != dst>>15 && src>>15 == uint16(out)>>15)
}

func xadd(cpu *CPU) {
	src := cpu.srcW()
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp)
	out := uint32(src) + uint32(dst)
	cpu.writeW(dp, uint16(out))
	cpu.PS.setNZ(uint16(out))
	cpu.PS.SetC(out>>16 != 0)
	cpu.PS.SetV(src>>15 == dst>>15 && src>>15 != uint16(out)>>15)
}

// logical instructons

func xbic(cpu *CPU) {
	src := cpu.srcW()
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp) &^ src
	cpu.writeW(dp, dst)
	cpu.PS.setNZ(dst)
	cpu.PS.SetV(false)
}

func xbicb(cpu *CPU) {
	src := cpu.srcB()
	dp := cpu.dstAddrB()
	dst := cpu.readB(dp) &^ src
	cpu.writeB(dp, dst)
	cpu.PS.setNZB(dst)
	cpu.PS.SetV(false)
}

func xbis(cpu *CPU) {
	src := cpu.srcW()
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp) | src
	cpu.writeW(dp, dst)
	cpu.PS.setNZ(dst)
	cpu.PS.SetV(false)
}

func xbisb(cpu *CPU) {
	src := cpu.srcB()
	dp := cpu.dstAddrB()
	dst := cpu.readB(dp) | src
	cpu.writeB(dp, dst)
	cpu.PS.setNZB(dst)
	cpu.PS.SetV(false)
}

func xbit(cpu *CPU) {
	src := cpu.srcW()
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp) & src
	cpu.PS.setNZ(dst)
	cpu.PS.SetV(false)
}

func xbitb(cpu *CPU) {
	src := cpu.srcB()
	dp := cpu.dstAddrB()
	dst := cpu.readB(dp) & src
	cpu.PS.setNZB(dst)
	cpu.PS.SetV(false)
}

// multiply and divide

func xmul(cpu *CPU) {
	r := cpu.regArg()
	src := cpu.dstW() // dst because low bits
	out := int32(int16(cpu.R[r])) * int32(int16(src))
	cpu.PS.SetN(out < 0)
	cpu.PS.SetZ(out == 0)
	cpu.PS.SetV(false)
	cpu.PS.SetC(false) // TODO int32(int16(out)) != out)
	cpu.R[r] = uint16(out >> 16)
	cpu.R[r|1] = uint16(out)
}

func xdiv(cpu *CPU) {
	r := cpu.regArg()
	if r&1 != 0 {
		panic(ErrInst) // divide with odd register
	}
	top := int32(cpu.R[r])<<16 | int32(cpu.R[r+1])
	src := cpu.dstW() // dst because low bits
	if src == 0 {
		cpu.PS.SetV(true)
		cpu.PS.SetC(true)
		cpu.PS.SetN(false)
		cpu.PS.SetZ(false)
		return
	}
	cpu.PS.SetC(false)
	q, rem := top/int32(int16(src)), top%int32(int16(src))

	// TODO remainder same sign as dividend
	cpu.R[r] = uint16(q)
	cpu.R[r+1] = uint16(rem)
	cpu.PS.SetN(q < 0)
	cpu.PS.SetZ(q == 0)
	cpu.PS.SetV(int32(int16(q)) != q)
}

func xash(cpu *CPU) {
	r := cpu.regArg()
	sh := int16(cpu.dstW()) << 10 >> 10 // dst because low bits
	v := cpu.R[r]
	old := v
	if sh < 0 {
		v = uint16(int16(v) >> -sh)
		cpu.PS.SetC((old>>(-sh-1))&1 != 0)
	} else if sh > 0 {
		v <<= sh
		cpu.PS.SetC((old<<(sh-1))>>15 != 0)
	} else {
		// shift 0
		cpu.PS.SetC(false)
	}
	cpu.PS.SetV(v>>15 != old>>15)
	cpu.PS.setNZ(v)
	cpu.R[r] = v
}

func xashc(cpu *CPU) {
	r := cpu.regArg()
	sh := int16(cpu.dstW()) << 10 >> 10 // dst because low bits
	v := uint32(cpu.R[r])<<16 | uint32(cpu.R[r|1])
	old := v
	if sh < 0 {
		v = uint32(int32(v) >> -sh)
		cpu.PS.SetC((old>>(-sh-1))&1 != 0)
	} else if sh > 0 {
		v <<= sh
		cpu.PS.SetC((old<<(sh-1))>>31 != 0)
	} else {
		// shift 0
		cpu.PS.SetC(false)
	}
	cpu.PS.SetV(v>>31 != old>>31)
	cpu.PS.SetN(v>>31 != 0)
	cpu.PS.SetZ(v == 0)
	cpu.R[r] = uint16(v >> 16)
	cpu.R[r|1] = uint16(v)
}

func xxor(cpu *CPU) {
	src := cpu.R[cpu.regArg()]
	dp := cpu.dstAddrW()
	dst := cpu.readW(dp) ^ src
	cpu.writeW(dp, dst)
	cpu.PS.setNZ(dst)
	cpu.PS.SetV(false)
}

// branches

func (cpu *CPU) br(c bool) {
	if c {
		cpu.R[PC] += 2 * uint16(int8(cpu.Inst))
	}
}

func xbr(cpu *CPU)   { cpu.br(true) }
func xbcc(cpu *CPU)  { cpu.br(cpu.PS.C() == 0) }
func xbcs(cpu *CPU)  { cpu.br(cpu.PS.C() == 1) }
func xbeq(cpu *CPU)  { cpu.br(cpu.PS.Z() == 1) }
func xbne(cpu *CPU)  { cpu.br(cpu.PS.Z() == 0) }
func xbpl(cpu *CPU)  { cpu.br(cpu.PS.N() == 0) }
func xbmi(cpu *CPU)  { cpu.br(cpu.PS.N() == 1) }
func xbvc(cpu *CPU)  { cpu.br(cpu.PS.V() == 0) }
func xbvs(cpu *CPU)  { cpu.br(cpu.PS.V() == 1) }
func xbge(cpu *CPU)  { cpu.br(cpu.PS.N()^cpu.PS.V() == 0) }
func xblt(cpu *CPU)  { cpu.br(cpu.PS.N()^cpu.PS.V() == 1) }
func xbgt(cpu *CPU)  { cpu.br(cpu.PS.Z()|(cpu.PS.N()^cpu.PS.V()) == 0) }
func xble(cpu *CPU)  { cpu.br(cpu.PS.Z()|(cpu.PS.N()^cpu.PS.V()) == 1) }
func xbhi(cpu *CPU)  { cpu.br(cpu.PS.C()|cpu.PS.Z() == 0) }
func xblos(cpu *CPU) { cpu.br(cpu.PS.C()|cpu.PS.Z() == 1) }

func xjmp(cpu *CPU) {
	dp := cpu.dstAddrW()
	if dp&addrReg != 0 {
		panic(ErrInst)
	}
	cpu.R[PC] = uint16(dp)
}

func xjsr(cpu *CPU) {
	r := cpu.regArg()
	dp := cpu.dstAddrW()
	if dp&addrReg != 0 {
		panic(ErrInst)
	}
	sp := cpu.R[SP] - 2
	cpu.R[SP] = sp
	cpu.writeW(addr(sp), cpu.R[r])
	cpu.R[r] = cpu.R[PC]
	cpu.R[PC] = uint16(dp)
}

func xrts(cpu *CPU) {
	r := RegNum(cpu.Inst & 07) // note: not cpu.regArg()
	sp := cpu.R[SP]
	cpu.R[PC] = cpu.R[r]
	cpu.R[r] = cpu.readW(addr(sp))
	cpu.R[SP] = sp + 2
}

func xsob(cpu *CPU) {
	r := cpu.regArg()
	if cpu.R[r]--; cpu.R[r] != 0 {
		cpu.R[PC] -= 2 * uint16(cpu.Inst&0o77)
	}
}

// special

func xtrap(cpu *CPU) {
	panic(ErrTrap)
}

func xbad(cpu *CPU) {
	panic(ErrInst)
}

func xbpt(cpu *CPU) { panic(ErrBPT) }

func xccc(cpu *CPU) {
	cpu.PS &^= PS(cpu.Inst & 0o17)
}

func xscc(cpu *CPU) {
	cpu.PS |= PS(cpu.Inst & 0o17)
}

func xemt(cpu *CPU)  { panic(ErrEMT) }
func xhalt(cpu *CPU) { panic(ErrInst) }

func xiot(cpu *CPU) { panic(ErrIOT) }

func xmark(cpu *CPU) { panic(ErrInst) }
func xmfpi(cpu *CPU) { panic(ErrInst) }
func xmtpi(cpu *CPU) { panic(ErrInst) }

func xreset(cpu *CPU) { panic(ErrInst) }
func xrti(cpu *CPU)   { panic(ErrInst) }
func xrtt(cpu *CPU)   { panic(ErrInst) }

func xwait(cpu *CPU) { panic(ErrInst) }
