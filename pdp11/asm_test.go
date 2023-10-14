// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func testDisasm(t *testing.T, do func(string, int, []uint16, string)) {
	const file = "testdata/disasm.txt"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	for i := 0; i < len(lines); {
		lineno := i + 1
		line, _, _ := strings.Cut(lines[i], "//")
		line = strings.TrimSpace(line)
		if line == "" {
			i++
			continue
		}
		code, text, ok := parseDisasm(line)
		if !ok {
			t.Fatalf("%s:%d: unexpected syntax", file, lineno)
		}
		codes := []uint16{code}
		i++
		for i < len(lines) {
			code, text, ok := parseDisasm(lines[i])
			if !ok || text != "" {
				break
			}
			codes = append(codes, code)
			i++
		}
		do(file, lineno, codes, text)
	}
}

func parseDisasm(line string) (code uint16, text string, ok bool) {
	num, text, _ := strings.Cut(line, " ")
	n, err := strconv.ParseUint(num, 8, 16)
	if err != nil {
		return 0, "", false
	}
	return uint16(n), text, true
}

func TestDisasm(t *testing.T) {
	var cpu CPU
	mem := new(ArrayMem)
	cpu.Mem = mem
	const basePC = 0o010000
	testDisasm(t, func(file string, line int, codes []uint16, text string) {
		for i := range mem {
			mem[i] = 0o375
		}
		for i, code := range codes {
			if err := cpu.Mem.WriteW(basePC+2*uint16(i), code); err != nil {
				t.Fatal(err)
			}
		}
		asm, next, err := cpu.Disasm(basePC)
		if err != nil {
			t.Fatalf("%s:%d: %v", file, line, err)
		}
		wantPC := basePC + 2*uint16(len(codes))
		if asm != text || next != wantPC {
			t.Errorf("%s:%d: Disasm() = %q, %v, want %q, %v", file, line, asm, next, text, wantPC)
		}
	})
}

func TestAsm(t *testing.T) {
	testDisasm(t, func(file string, line int, codes []uint16, text string) {
		const basePC = 0o010000
		acodes, err := Asm(basePC, text)
		if err != nil {
			t.Fatalf("%s:%d: %v", file, line, err)
		}
		if !reflect.DeepEqual(acodes, codes) {
			t.Errorf("%s:%d: Asm(%q) = %06o, want %06o", file, line, text, acodes, codes)
		}
	})

}

func TestDisasmAsm(t *testing.T) {
	var cpu CPU
	mem := new(ArrayMem)
	cpu.Mem = mem
	for i := range mem {
		mem[i] = 0o375
	}
	errs := 0
	const basePC = 0o010000
	for i := 0; i < 1<<16; i++ {
		codes := []uint16{uint16(i), 0o100, 0o200, 0o300}
		for i, code := range codes {
			if err := cpu.Mem.WriteW(basePC+2*uint16(i), code); err != nil {
				t.Fatal(err)
			}
		}
		asm, nextPC, err := cpu.Disasm(basePC)
		if err != nil {
			continue
		}
		codes = codes[:(nextPC-basePC)/2]
		acodes, err := Asm(basePC, asm)
		if err != nil {
			t.Errorf("Disasm(%06o) = %q, but Asm failed: %v", codes, asm, err)
			if errs++; errs >= 20 {
				t.Fatalf("too many errors")
			}
		}
		if !reflect.DeepEqual(acodes, codes) {
			t.Errorf("Disasm(%06o) = %q, but Asm(%q) = %06o", codes, asm, asm, acodes)
			if errs++; errs >= 20 {
				t.Fatalf("too many errors")
			}
		}
	}
}
