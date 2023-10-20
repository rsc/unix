// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestExec(t *testing.T) {
	files, err := filepath.Glob("testdata/exec*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".txt")
		t.Run(name, func(t *testing.T) {
			testExec(t, file)
		})

		switch name {
		case "exec_br", "exec_jmp", "exec_jsr", "exec_sob":
			// skip
		default:
			t.Run(name+"_apout", func(t *testing.T) {
				testApoutExec(t, file)
			})
		}
	}
}

func testExec(t *testing.T, file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")

	var cpu CPU
	mem := new(ArrayMem)
	cpu.Mem = mem

	const basePC = 0o010000
	regs := [8]uint16{0, 0, 0, 0, 0, 0, 0, basePC}
	codes := []uint16{}

	reset := func() {
		for i := 0; i < 1<<16; i += 2 {
			mem.WriteW(uint16(i), 070707)
		}
		regs[PC] = basePC
		cpu.R = regs
		clear(cpu.F[:])
		cpu.PS = 0
		cpu.FPS = 0
		codes = codes[:0]
	}

	diff := func() string {
		regs[PC] = basePC + 2*uint16(len(codes))
		var list []string
		for i := RegNum(0); i <= PC; i++ {
			if cpu.R[i] != regs[i] {
				list = append(list, fmt.Sprintf("%v=%06o", i, cpu.R[i]))
			}
		}
		if cpu.PS != 0 {
			list = append(list, fmt.Sprintf("nzvc=%04b", int(cpu.PS)))
		}
		for i := range cpu.F {
			if f := cpu.F[i]; math.Float64bits(f) != 0 {
				list = append(list, fmt.Sprintf("f%d=%v", i, f))
			}
		}
		if cpu.FPS != 0 {
			list = append(list, fmt.Sprintf("fps=%v", cpu.FPS))
		}
		for i := 0; i < 1<<16; i += 2 {
			want := uint16(070707)
			if basePC <= i && i < basePC+2*int(len(codes)) {
				want = codes[(i-basePC)/2]
			}
			x, _ := mem.ReadW(uint16(i))
			if x != want {
				list = append(list, fmt.Sprintf("*%06o=%06o", i, x))
			}
		}
		if len(list) == 0 {
			list = append(list, "~")
		}
		return strings.Join(list, " ")
	}

	reset()
	var broken bool
	var last string
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			broken = false
			last = "reset"
			reset()
			continue
		}
		if broken {
			continue
		}
		line, _, _ = strings.Cut(line, "//")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if f[0] == "now" {
			have := "now " + diff()
			if have != line {
				t.Errorf("%s:%d: after %s:\nhave %s\nwant %s", file, i+1, last, have, line)
				broken = true
			}
			continue
		}
		if f[0] == "set" {
			for _, arg := range f[1:] {
				k, v, ok := strings.Cut(arg, "=")
				if ok {
					ok = false
					switch k {
					case "f0", "f1", "f2", "f3", "f4", "f5":
						f, err := strconv.ParseFloat(v, 64)
						if err == nil {
							cpu.F[k[1]-'0'] = f
							ok = true
						}
					}
				}
				if !ok {
					t.Errorf("%s:%d: bad set: %s", file, i+1, arg)
					broken = true
					break
				}
			}
			continue
		}

		pc := basePC + 2*uint16(len(codes))
		if cpu.R[PC] != pc {
			t.Errorf("%s:%d: after %s: PC=%06o want %06o", file, i+1, last, cpu.R[PC], pc)
			broken = true
			continue
		}
		acodes, err := Asm(pc, line)
		if err != nil {
			t.Errorf("%s:%d: %s: %v", file, i+1, line, err)
			broken = true
			continue
		}
		last = line
		old := len(codes)
		codes = append(codes, acodes...)
		for i := old; i < len(codes); i++ {
			mem.WriteW(basePC+2*uint16(i), codes[i])
		}
		if err := cpu.Step(1); err != nil {
			t.Errorf("%s:%d: %s: %v", file, i+1, line, err)
			broken = true
			continue
		}
	}
}

func testApoutExec(t *testing.T, file string) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	root := os.Getenv("APOUT_ROOT")
	if root == "" {
		t.Skip("$APOUT_ROOT not set")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")

	type Now struct {
		pc   uint16
		text string
		pos  string
	}
	pc := uint16(0)
	var codes []uint16
	asm := func(line string) {
		acodes, err := Asm(pc+2*uint16(len(codes)), line)
		if err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		codes = append(codes, acodes...)
	}
	asm("br 4")
	codes = append(codes, 0o010600) // v6 crt0 magic for apout

	var nows []Now
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			asm("mov #0, r0")
			asm("mov r0, r1")
			asm("mov r0, r2")
			asm("mov r0, r3")
			asm("mov r0, r4")
			asm("mov r0, r5")
			asm("mov r0, r6")
			asm("ccc")
			continue
		}
		line, _, _ = strings.Cut(line, "//")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "now ") {
			nows = append(nows, Now{pc + 2*uint16(len(codes)), strings.TrimPrefix(line, "now "), fmt.Sprintf("%s:%d", file, i+1)})
			line = "nop"
		}
		asm(line)
	}
	asm("trap 1") // exit
	asm("halt")

	if len(codes) > 32000 {
		t.Fatalf("too many instructions %d", len(codes))
	}

	aout := make([]byte, (8+len(codes))*2)

	put := binary.LittleEndian.PutUint16
	put(aout[0:], 0o000407)
	put(aout[2:], 2*uint16(len(codes)))
	put(aout[6:], 1)
	for i, code := range codes {
		put(aout[0o20+2*i:], code)
	}

	// write a.out file to temp executable
	dir := t.TempDir()
	exe := filepath.Join(dir, "a.out")
	if err := os.WriteFile(exe, aout, 0666); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("apout", "-inst", "-trap", "a.out")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		data, _ = os.ReadFile(filepath.Join(dir, "apout.dbg"))
		os.WriteFile("/tmp/apout.dbg", data, 0666)
		t.Fatalf("apout: %v\n%s", err, out)
	}

	data, err = os.ReadFile(filepath.Join(dir, "apout.dbg"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile("/tmp/apout.dbg", data, 0666)
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		// 000240 ccc is the nop
		if len(f) == 12 && f[10] == "NZVC1" && f[1] == "000240" && f[2] == "ccc" {
			pc64, err := strconv.ParseUint(f[0], 8, 16)
			if err != nil {
				t.Fatalf("bad apout.dbg line: %s", line)
			}
			pc := uint16(pc64)
			if len(nows) == 0 || pc < nows[0].pc {
				continue
			}
			if pc != nows[0].pc {
				t.Fatalf("have pc %06o, want %06o", pc, nows[0].pc)
			}
			diff := ""
			for i := 0; i < 6; i++ {
				if f[3+i] != "000000" {
					diff += fmt.Sprintf(" r%d=%s", i, f[3+i])
				}
			}
			if f[11] != "0000" {
				diff += " nzvc=" + f[11]
			}
			if diff == "" {
				diff = " ~"
			}
			diff = diff[1:]
			if diff != nows[0].text {
				t.Errorf("%s:\nhave %s\nwant %s", nows[0].pos, diff, nows[0].text)
			}
			nows = nows[1:]
		}
	}
	if len(nows) > 0 {
		t.Fatalf("did not see pc %06o", nows[0].pc)
	}
}
