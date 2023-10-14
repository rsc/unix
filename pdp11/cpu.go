// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import (
	"fmt"
)

// A CPU represents a single PDP-11 CPU, connected to a memory.
type CPU struct {
	R    [8]uint16 // registers
	PS   PS        // processor status word
	Inst uint16    // instruction being executed (actual instruction bits)
	Mem  Memory    // attached memory
}

var (
	ErrMem  = fmt.Errorf("invalid memory access")
	ErrTrap = fmt.Errorf("trap")
	ErrInst = fmt.Errorf("invalid instruction")
)

// A Memory represents a PDP-11 memory.
type Memory interface {
	ReadB(addr uint16) (uint8, error)
	ReadW(addr uint16) (uint16, error)
	WriteB(addr uint16, val uint8) error
	WriteW(addr uint16, val uint16) error
}

// An ArrayMem is a Memory implementation backed by a 64kB array.
// All addresses are valid.
type ArrayMem [1 << 16]byte

func (m *ArrayMem) ReadB(addr uint16) (uint8, error) {
	return m[addr], nil
}

func (m *ArrayMem) ReadW(addr uint16) (uint16, error) {
	return uint16(m[addr]) | uint16(m[addr+1])<<8, nil
}

func (m *ArrayMem) WriteB(addr uint16, val uint8) error {
	m[addr] = val
	return nil
}

func (m *ArrayMem) WriteW(addr uint16, val uint16) error {
	m[addr] = uint8(val)
	m[addr+1] = uint8(val >> 8)
	return nil
}

// A RegNum is a register number (0..7).
type RegNum uint8

const (
	PC RegNum = 7 // R7 is program counter
	SP RegNum = 6 // R6 is stack pointer
)

// String returns the register name for r: r0, r1, r2, r3, r4, r5, sp, or pc.
func (r RegNum) String() string {
	if r == SP {
		return "sp"
	}
	if r == PC {
		return "pc"
	}
	return fmt.Sprintf("r%d", r)
}

// A PS is the processor status word.
// Only the condition codes are used.
type PS uint16

const (
	PS_C PS = 1 << 0 // C = 1 if result generated carry
	PS_V PS = 1 << 1 // V = 1 if result overflowed
	PS_Z PS = 1 << 2 // Z =1 if result was zero
	PS_N PS = 1 << 3 // N = 1 if result was negative
)

// C returns the carry bit as a uint16 that is 0 or 1.
func (p PS) C() uint16 { return uint16(p) & 1 }

// V returns the overflow bit as a uint16 that is 0 or 1.
func (p PS) V() uint16 { return (uint16(p) >> 1) & 1 }

// Z returns the zero bit as a uint16 that is 0 or 1.
func (p PS) Z() uint16 { return (uint16(p) >> 2) & 1 }

// N returns the sign (negative) bit as a uint16 that is 0 or 1.
func (p PS) N() uint16 { return (uint16(p) >> 3) & 1 }

// set sets the given bit to the bool value b.
func (p *PS) set(b bool, bit PS) {
	if b {
		*p |= bit
	} else {
		*p &^= bit
	}
}

// SetC sets the carry bit according to the boolean value.
func (p *PS) SetC(b bool) { p.set(b, PS_C) }

// SetV sets the overflow bit according to the boolean value.
func (p *PS) SetV(b bool) { p.set(b, PS_V) }

// SetZ sets the zero bit according to the boolean value.
func (p *PS) SetZ(b bool) { p.set(b, PS_Z) }

// SetN sets the sign (negative) bit according to the boolean value.
func (p *PS) SetN(b bool) { p.set(b, PS_N) }

// setNZ sets the sign and zero bits according to the value.
func (p *PS) setNZ(v uint16) {
	p.set(v == 0, PS_Z)
	p.set(v>>15 != 0, PS_N)
}

// setNZB sets the sign and zero bits according to the (byte) value.
func (p *PS) setNZB(v uint8) {
	p.set(v == 0, PS_Z)
	p.set(v>>7 != 0, PS_N)
}

const psAddr = 0o177776 // PS is at special address 0o177776

// ReadB reads and returns the byte at addr.
func (cpu *CPU) ReadB(addr uint16) (uint8, error) {
	if addr == psAddr {
		return uint8(cpu.PS), nil
	}
	return cpu.Mem.ReadB(addr)
}

// ReadW reads and returns the word at addr.
func (cpu *CPU) ReadW(addr uint16) (uint16, error) {
	// PS is at special address 0o177776.
	if addr == psAddr {
		return uint16(cpu.PS), nil
	}
	return cpu.Mem.ReadW(addr)
}

// WriteB writes the byte val to addr.
func (cpu *CPU) WriteB(addr uint16, val uint8) error {
	// PS is at special address 0o177776.
	if addr == psAddr {
		cpu.PS = PS(val)
		return nil
	}
	return cpu.Mem.WriteB(addr, val)
}

// WriteW writes the word val to addr.
func (cpu *CPU) WriteW(addr uint16, val uint16) error {
	// PS is at special address 0o177776.
	if addr == psAddr {
		cpu.PS = PS(val)
		return nil
	}
	return cpu.Mem.WriteW(addr, val)
}
