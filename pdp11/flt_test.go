// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import "testing"

var convTests = []struct {
	f    float64
	bits uint64
}{
	{32.0, 0b0_10000110_000_0000 << 48},
	{7 / 16.0, 0b0_11111111_10_0000 << 48},
	{0, 0},
}

func TestFloatConv(t *testing.T) {
	for _, tt := range convTests {
		b := tt.bits
		if f := fromF64(uint16(b>>48), uint16(b>>32), uint16(b>>16), uint16(b)); f != tt.f {
			t.Errorf("fromF64(%#016x) = %v, want %v", b, f, tt.f)
		}
		w0, w1, w2, w3 := toF64(tt.f)
		if b := uint64(w0)<<48 | uint64(w1)<<32 | uint64(w2)<<16 | uint64(w3); b != tt.bits {
			t.Errorf("toF64(%v) = %#016x, want %#016x", tt.f, b, tt.bits)
		}
		if b>>32<<32 == b {
			if f := fromF32(uint16(b>>48), uint16(b>>32)); f != tt.f {
				t.Errorf("fromF32(%#08x) = %v, want %v", b>>32, f, tt.f)
			}
			w0, w1 := toF32(tt.f)
			if b := uint64(w0)<<48 | uint64(w1)<<32; b != tt.bits {
				t.Errorf("toF32(%v) = %#08x, want %#08x", tt.f, b>>32, tt.bits>>32)
			}
		}
	}
}
