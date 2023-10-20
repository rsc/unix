// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import (
	"math"
	"strings"
)

const (
	minFloat = 0.5 / (1 << 0o177)
	maxFloat = float64(1<<56-1) / (1 << 56) * (1 << 0o177)
)

// An FPS is the processor status word.
// Only the condition codes are used.
type FPS uint16

const (
	FC FPS = 1 << iota // C = 1 if result generated carry
	FV                 // V = 1 if result overflowed
	FZ                 // Z =1 if result was zero
	FN                 // N = 1 if result was negative
	_
	FT   // truncate bit
	FL   // long-precision integer mode
	FD   // double-precision mode
	FIC  // floating interrupt on integer conversion error (TODO)
	FIV  // floating interrupt on overflow (TODO)
	FIU  // floating interrupt on underflow (TODO)
	FIUV // floating interrupt on undefined variable (TODO)
	_
	_
	FID // floating interrupt disable (TODO)
	FER // floating error condiiton present (TODO)
)

var ftab = []string{
	"c",
	"v",
	"z",
	"n",
	"4",
	"t",
	"l",
	"d",
	"ic,",
	"iv,",
	"iu,",
	"iuv,",
	"12,",
	"13,",
	"id,",
	"er,",
}

func (p FPS) String() string {
	text := ""
	for i := len(ftab) - 1; i >= 0; i-- {
		if p&(1<<i) != 0 {
			text += ftab[i]
		}
	}
	if text == "" {
		return "-"
	}
	return strings.TrimSuffix(text, ",")
}

// C returns the carry bit as a uint16 that is 0 or 1.
func (p FPS) C() uint16 { return uint16(p) & 1 }

// V returns the overflow bit as a uint16 that is 0 or 1.
func (p FPS) V() uint16 { return (uint16(p) >> 1) & 1 }

// Z returns the zero bit as a uint16 that is 0 or 1.
func (p FPS) Z() uint16 { return (uint16(p) >> 2) & 1 }

// N returns the sign (negative) bit as a uint16 that is 0 or 1.
func (p FPS) N() uint16 { return (uint16(p) >> 3) & 1 }

// set sets the given bit to the bool value b.
func (p *FPS) set(b bool, bit FPS) {
	if b {
		*p |= bit
	} else {
		*p &^= bit
	}
}

// SetC sets the carry bit according to the boolean value.
func (p *FPS) SetC(b bool) { p.set(b, FC) }

// SetV sets the overflow bit according to the boolean value.
func (p *FPS) SetV(b bool) { p.set(b, FV) }

// SetZ sets the zero bit according to the boolean value.
func (p *FPS) SetZ(b bool) { p.set(b, FZ) }

// SetN sets the sign (negative) bit according to the boolean value.
func (p *FPS) SetN(b bool) { p.set(b, FN) }

func (cpu *CPU) clamp(f float64) float64 {
	return cpu.conv(f, cpu.FPS&FT == 0)
}

func (cpu *CPU) conv(f float64, round bool) float64 {
	a := math.Abs(f)
	if a < minFloat {
		return 0
	}
	if cpu.FPS&FD == 0 {
		// ieee float64 has 64-11-1 = 52-bit mantissa, pdp11 float32 has 32-8-1 = 23-bit, clear 29
		bits := math.Float64bits(f)
		low := bits & (1<<29 - 1)
		bits &^= 1<<29 - 1
		if round {
			if low > 1<<28 {
				// round up
				bits += 1 << 29
			} else if low == 1<<28 {
				// round to even
				bits += bits & (1 << 29)
			}
		}
		// TODO check for overflow
		f = math.Float64frombits(bits)
	}
	return f
}

func fromF32(w0, w1 uint16) float64 {
	w := uint64(w0)<<16 | uint64(w1)
	sign := w >> 31 << 63
	exp := int64(w>>23)&0o377 - 0o200
	if exp == -0o200 {
		return 0
	}
	mant := w & (1<<23 - 1) << (52 - 23)
	return math.Float64frombits(sign | uint64(exp+1022)<<52 | mant)
}

func fromF64(w0, w1, w2, w3 uint16) float64 {
	w := uint64(w0)<<48 | uint64(w1)<<32 | uint64(w2)<<16 | uint64(w3)
	sign := w >> 63 << 63
	exp := int64(w>>55)&0o377 - 0o200
	if exp == -0o200 {
		return 0
	}
	mant := w & (1<<55 - 1)
	// round to even before shift down 55-52 = 3
	if mant&(1<<2) != 0 { // TODO wrong
		mant += mant & (1 << 3)
	}
	if mant >= 1<<55 {
		mant >>= 1
		exp++
	}
	mant >>= 3
	return math.Float64frombits(sign | uint64(exp+1022)<<52 | mant)
}

func toF32(f float64) (w0, w1 uint16) {
	b := math.Float64bits(f)
	sign := uint32(b >> 63 << 31)
	exp := int(b>>52)&2047 - 1022
	mant := b & (1<<52 - 1)
	if exp < -0o200 {
		return
	}
	if exp >= 0o200 {
		return // TODO what
	}
	// round to even before shift down 52-23 = 29
	if mant&(1<<28) != 0 { // TODO wrong
		mant += mant & (1 << 29)
	}
	if mant >= 1<<52 {
		mant >>= 1
		exp++
	}
	mant >>= 29
	w := sign | uint32(exp+0o200)<<23 | uint32(mant)
	return uint16(w >> 16), uint16(w)
}

func toF64(f float64) (w0, w1, w2, w3 uint16) {
	b := math.Float64bits(f)
	sign := b >> 63 << 63
	exp := int(b>>52)&2047 - 1022
	mant := b & (1<<52 - 1)
	if exp < -0o200 {
		return
	}
	if exp >= 0o200 {
		return // TODO what
	}
	mant <<= 3
	w := sign | uint64(exp+0o200)<<55 | mant
	return uint16(w >> 48), uint16(w >> 32), uint16(w >> 16), uint16(w)
}

func (cpu *CPU) readF(a addr) float64 {
	if a&addrReg != 0 {
		if int(a&07) >= len(cpu.F) {
			panic(1)
			panic(ErrInst)
		}
		return cpu.conv(cpu.F[a&07], false)
	}
	w0, err := cpu.Mem.ReadW(uint16(a))
	if err != nil {
		panic(err)
	}
	if regOrImm(cpu) {
		return fromF32(w0, 0)
	}
	w1, err := cpu.Mem.ReadW(uint16(a + 2))
	if err != nil {
		panic(err)
	}
	if cpu.FPS&FD == 0 {
		return fromF32(w0, w1)
	}
	w2, err := cpu.Mem.ReadW(uint16(a + 4))
	if err != nil {
		panic(err)
	}
	w3, err := cpu.Mem.ReadW(uint16(a + 6))
	if err != nil {
		panic(err)
	}
	return fromF64(w0, w1, w2, w3)
}

func (cpu *CPU) writeF(a addr, f float64) {
	if a&addrReg != 0 {
		if int(a&07) >= len(cpu.F) {
			panic(2)
			panic(ErrInst)
		}
		cpu.F[a&07] = f
		return
	}
	if cpu.FPS&FD == 0 {
		w0, w1 := toF32(f)
		if err := cpu.Mem.WriteW(uint16(a), w0); err != nil {
			panic(err)
		}
		if err := cpu.Mem.WriteW(uint16(a+2), w1); err != nil {
			panic(err)
		}
		return
	}

	w0, w1, w2, w3 := toF64(f)
	if err := cpu.Mem.WriteW(uint16(a), w0); err != nil {
		panic(err)
	}
	if err := cpu.Mem.WriteW(uint16(a+2), w1); err != nil {
		panic(err)
	}
	if err := cpu.Mem.WriteW(uint16(a+4), w2); err != nil {
		panic(err)
	}
	if err := cpu.Mem.WriteW(uint16(a+6), w3); err != nil {
		panic(err)
	}
}

func (cpu *CPU) ax() int {
	return int(cpu.Inst>>6) & 03
}

func (cpu *CPU) srcF() float64 {
	return cpu.readF(cpu.dstAddrF())
}

func xsetf(cpu *CPU) {
	cpu.FPS &^= FD
}

func xseti(cpu *CPU) {
	cpu.FPS &^= FL
}

func xsetd(cpu *CPU) {
	cpu.FPS |= FD
}

func xsetl(cpu *CPU) {
	cpu.FPS |= FL
}

func xabsf(cpu *CPU) {
	fp := cpu.dstAddrF()
	f := cpu.readF(fp)
	if f < 0 {
		f = -f
	}
	if f == 0 {
		f = 0 // clear sign
	}
	f = math.Abs(f)
	cpu.writeF(fp, f)
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(false)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(false)
}

func xaddf(cpu *CPU) {
	f := cpu.srcF()
	ax := cpu.ax()
	a := cpu.F[ax] + f
	a = cpu.clamp(a)
	cpu.F[ax] = a
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(math.Abs(a) >= maxFloat)
	cpu.FPS.SetZ(a == 0)
	cpu.FPS.SetN(a < 0)
}

func xclrf(cpu *CPU) {
	fp := cpu.dstAddrF()
	cpu.writeF(fp, 0)
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(false)
	cpu.FPS.SetZ(true)
	cpu.FPS.SetN(false)
}

func xcmpf(cpu *CPU) {
	f := cpu.srcF() - cpu.F[cpu.ax()]
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(false)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func xsubf(cpu *CPU) {
	ax := cpu.ax()
	f := cpu.F[ax] - cpu.srcF()
	cpu.F[ax] = f
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(math.Abs(f) >= maxFloat)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func xcfcc(cpu *CPU) {
	cpu.PS = cpu.PS&^0o17 | PS(cpu.FPS&0o17)
}

func xdivf(cpu *CPU) {
	ax := cpu.ax()
	f := cpu.F[ax] / cpu.srcF()
	f = cpu.clamp(f)
	cpu.F[ax] = f
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(math.Abs(f) >= maxFloat)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func xldf(cpu *CPU) {
	f := cpu.srcF()
	cpu.F[cpu.ax()] = f
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(false)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func xldcdf(cpu *CPU) {
	cpu.FPS ^= FD
	f := cpu.srcF()
	cpu.FPS ^= FD
	f = cpu.clamp(f)
	cpu.F[cpu.ax()] = f
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(math.Abs(f) >= maxFloat)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func regOrImm(cpu *CPU) bool {
	return cpu.Inst&070 == 000 || cpu.Inst&077 == 027
}

func xldcif(cpu *CPU) {
	var f float64
	if cpu.FPS&FL == 0 {
		// 16-bit value
		f = float64(int16(cpu.dstW()))
	} else {
		// 32-bit value
		var i int32
		if regOrImm(cpu) {
			w := cpu.dstW()
			i = int32(w) << 16
		} else {
			dp := cpu.dstAddrF()
			i = int32(cpu.readW(dp))<<16 | int32(cpu.readW(dp+2))
		}
		f = float64(i)
	}
	f = cpu.clamp(f)
	cpu.F[cpu.ax()] = f
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(false)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func xldexp(cpu *CPU) {
	ax := cpu.ax()
	f := cpu.F[cpu.ax()]
	src := int16(cpu.dstW())
	if -0o177 <= src && src <= 0o177 {
		f = math.Ldexp(f, int(src))
		cpu.F[ax] = f
		cpu.FPS.SetV(false)
	} else {
		cpu.FPS.SetV(true)
	}
	cpu.FPS.SetC(false)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func xldfps(cpu *CPU) {
	cpu.FPS = FPS(cpu.dstW())
}

func xmodf(cpu *CPU) {
	f := cpu.srcF()
	ax := cpu.ax()
	a := cpu.F[ax] * f
	a = cpu.clamp(a)
	int, frac := math.Modf(a)
	cpu.F[ax|1] = int
	frac = cpu.clamp(frac)
	cpu.F[ax] = frac
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(math.Abs(frac) >= maxFloat) // always false?
	cpu.FPS.SetZ(frac == 0)
	cpu.FPS.SetN(frac < 0)
}

func xmulf(cpu *CPU) {
	f := cpu.srcF()
	ax := cpu.ax()
	a := cpu.F[ax] * f
	a = cpu.clamp(a)
	cpu.F[ax] = a
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(math.Abs(a) >= maxFloat)
	cpu.FPS.SetZ(a == 0)
	cpu.FPS.SetN(a < 0)
}

func xnegf(cpu *CPU) {
	fp := cpu.dstAddrF()
	f := cpu.readF(fp)
	f = -f
	if f == 0 {
		f = 0 // clear sign
	}
	cpu.writeF(fp, f)
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(false)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func xstf(cpu *CPU) {
	fp := cpu.dstAddrF()
	f := cpu.F[cpu.ax()]
	cpu.writeF(fp, f)
	/*
		cpu.FPS.SetC(false)
		cpu.FPS.SetV(false)
		cpu.FPS.SetZ(f == 0)
		cpu.FPS.SetN(f < 0)
	*/
}

func xstcfd(cpu *CPU) {
	f := cpu.F[cpu.ax()]
	f = cpu.clamp(f)
	cpu.FPS ^= FD
	f = cpu.clamp(f)
	cpu.writeF(cpu.dstAddrF(), f)
	cpu.FPS ^= FD

	cpu.FPS.SetC(false)
	cpu.FPS.SetV(math.Abs(f) >= maxFloat)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}

func xstcfi(cpu *CPU) {
	f := math.Round(cpu.F[cpu.ax()])
	cpu.FPS.SetC(false)
	cpu.PS.SetC(false)
	if cpu.FPS&FL == 0 {
		dp := cpu.dstAddrW()
		if float64(int16(f)) != f {
			f = 0
			cpu.FPS.SetC(true)
			cpu.PS.SetC(true)
		}
		cpu.writeW(dp, uint16(int16(f)))
	} else {
		dp := cpu.dstAddrL()
		if float64(int32(f)) != f {
			f = 0
			cpu.FPS.SetC(true)
			cpu.PS.SetC(true)
		}
		i := uint32(int32(f))
		cpu.writeW(dp, uint16(i>>16))
		if !regOrImm(cpu) {
			cpu.writeW(dp+2, uint16(i))
		}
	}
	cpu.FPS.SetV(false)
	cpu.PS.SetV(false)
	cpu.FPS.SetZ(f == 0)
	cpu.PS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
	cpu.PS.SetN(f < 0)
}

func xstexp(cpu *CPU) {
	f := cpu.F[cpu.ax()]
	_, exp := math.Frexp(f)
	cpu.writeW(cpu.dstAddrF(), uint16(exp-1))
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(false)
	cpu.FPS.SetZ(exp == 0)
	cpu.FPS.SetN(exp < 0)
	cpu.FPS = cpu.FPS
}

func xstfps(cpu *CPU) {
	cpu.writeW(cpu.dstAddrW(), uint16(cpu.FPS))
}

func xstst(cpu *CPU) {
	panic("stst") // FEC and FEA not set yet
	dp := cpu.dstAddrL()
	cpu.writeW(dp, uint16(cpu.FEC))
	if !regOrImm(cpu) {
		cpu.writeW(dp+2, uint16(cpu.FEA))
	}
}

func xtstf(cpu *CPU) {
	f := cpu.srcF()
	cpu.FPS.SetC(false)
	cpu.FPS.SetV(false)
	cpu.FPS.SetZ(f == 0)
	cpu.FPS.SetN(f < 0)
}
