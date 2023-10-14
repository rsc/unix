// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pdp11

import (
	"sort"
	"strings"
)

type instr struct {
	code uint16
	do   func(cpu *CPU)
	text string
}

var itab = []instr{
	{0o000000, xhalt, "halt"},   // untested
	{0o000001, xwait, "wait"},   // untested
	{0o000002, xrti, "rti"},     // untested
	{0o000003, xbpt, "bpt"},     // untested
	{0o000004, xiot, "iot"},     // untested
	{0o000005, xreset, "reset"}, // untested
	{0o000006, xrtt, "rtt"},     // untested
	{0o000007, xbad, ""},
	{0o000100, xjmp, "jmp %d"},
	{0o000200, xrts, "rts %R"},
	{0o000210, xbad, ""},
	{0o000240, xccc, "nop"},
	{0o000241, xccc, "clc"},
	{0o000242, xccc, "clv"},
	{0o000243, xccc, "clvc"},
	{0o000244, xccc, "clz"},
	{0o000245, xccc, "clzc"},
	{0o000246, xccc, "clzv"},
	{0o000247, xccc, "clzvc"},
	{0o000250, xccc, "cln"},
	{0o000251, xccc, "clnc"},
	{0o000252, xccc, "clnv"},
	{0o000253, xccc, "clnvc"},
	{0o000254, xccc, "clnz"},
	{0o000255, xccc, "clnzc"},
	{0o000256, xccc, "clnzv"},
	{0o000257, xccc, "ccc"},
	{0o000260, xscc, "snop"},
	{0o000261, xscc, "sec"},
	{0o000262, xscc, "sev"},
	{0o000263, xscc, "sevc"},
	{0o000264, xscc, "sez"},
	{0o000265, xscc, "sezc"},
	{0o000266, xscc, "sezv"},
	{0o000267, xscc, "sezvc"},
	{0o000270, xscc, "sen"},
	{0o000271, xscc, "senc"},
	{0o000272, xscc, "senv"},
	{0o000273, xscc, "senvc"},
	{0o000274, xscc, "senz"},
	{0o000275, xscc, "senzc"},
	{0o000276, xscc, "senzv"},
	{0o000277, xscc, "scc"},
	{0o000300, xswab, "swab %d"},
	{0o000400, xbr, "br %b"},
	{0o001000, xbne, "bne %b"},
	{0o001400, xbeq, "beq %b"},
	{0o002000, xbge, "bge %b"},
	{0o002400, xblt, "blt %b"},
	{0o003000, xbgt, "bgt %b"},
	{0o003400, xble, "ble %b"},
	{0o004000, xjsr, "jsr %r, %d"},
	{0o005000, xclr, "clr %d"},
	{0o005100, xcom, "com %d"},
	{0o005200, xinc, "inc %d"},
	{0o005300, xdec, "dec %d"},
	{0o005400, xneg, "neg %d"},
	{0o005500, xadc, "adc %d"},
	{0o005600, xsbc, "sbc %d"},
	{0o005700, xtst, "tst %d"},
	{0o006000, xror, "ror %d"},
	{0o006100, xrol, "rol %d"},
	{0o006200, xasr, "asr %d"},
	{0o006300, xasl, "asl %d"},
	{0o006400, xmark, "mark %d"}, // untested
	{0o006500, xmfpi, "mfpi %d"}, // untested
	{0o006600, xmtpi, "mtpi %d"}, // untested
	{0o006700, xsxt, "sxt %d"},
	{0o007000, xbad, ""},
	{0o010000, xmov, "mov %s, %d"},
	{0o020000, xcmp, "cmp %s, %d"},
	{0o030000, xbit, "bit %s, %d"},
	{0o040000, xbic, "bic %s, %d"},
	{0o050000, xbis, "bis %s, %d"},
	{0o060000, xadd, "add %s, %d"},
	{0o070000, xmul, "mul %d, %r"},
	{0o071000, xdiv, "div %d, %r"},
	{0o072000, xash, "ash %d, %r"},
	{0o073000, xashc, "ashc %d, %r"},
	{0o074000, xxor, "xor %r, %d"},
	{0o075000, xbad, ""},
	{0o077000, xsob, "sob %r, %B"},
	{0o100000, xbpl, "bpl %b"},
	{0o100400, xbmi, "bmi %b"},
	{0o101000, xbhi, "bhi %b"},
	{0o101400, xblos, "blos %b"},
	{0o102000, xbvc, "bvc %b"},
	{0o102400, xbvs, "bvs %b"},
	{0o103000, xbcc, "bcc %b"},
	{0o103400, xbcs, "bcs %b"},
	{0o104000, xemt, "emt %n"},   // untested
	{0o104400, xtrap, "trap %n"}, // untested
	{0o105000, xclrb, "clrb %d"},
	{0o105100, xcomb, "comb %d"},
	{0o105200, xincb, "incb %d"},
	{0o105300, xdecb, "decb %d"},
	{0o105400, xnegb, "negb %d"},
	{0o105500, xadcb, "adcb %d"},
	{0o105600, xsbcb, "sbcb %d"},
	{0o105700, xtstb, "tstb %d"},
	{0o106000, xrorb, "rorb %d"},
	{0o106100, xrolb, "rolb %d"},
	{0o106200, xasrb, "asrb %d"},
	{0o106300, xaslb, "aslb %d"},
	{0o106400, xbad, ""},
	{0o110000, xmovb, "movb %s, %d"},
	{0o120000, xcmpb, "cmpb %s, %d"},
	{0o130000, xbitb, "bitb %s, %d"},
	{0o140000, xbicb, "bicb %s, %d"},
	{0o150000, xbisb, "bisb %s, %d"},
	{0o160000, xsub, "sub %s, %d"},
	{0o170000, xbad, ""},
	{0o170011, xsetd, "setd"}, // untested
	{0o170012, xbad, ""},
}

func lookup(inst uint16) *instr {
	i := sort.Search(len(itab), func(i int) bool { return itab[i].code > inst }) - 1
	return &itab[i]
}

func lookupAsm(op string) *instr {
	for i := range itab {
		inst := &itab[i]
		if iop, _, _ := strings.Cut(inst.text, " "); iop == op {
			return inst
		}
	}
	return nil
}
