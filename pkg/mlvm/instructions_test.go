// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package mlvm_test

import (
	"github.com/maloquacious/ml_i/pkg/mlvm"
	"testing"
)

func TestEncodeABC(t *testing.T) {
	type scenario struct {
		id     int
		input  mlvm.IABC
		expect mlvm.Word
	}
	for _, tc := range []scenario{
		{1, mlvm.IABC{}, mlvm.Word(0)},
		{2, mlvm.IABC{Op: 0x01}, mlvm.Word(0x0400_0000)},
		{3, mlvm.IABC{Op: 0x3f}, mlvm.Word(0xfc00_0000)},
		{4, mlvm.IABC{A: 0xff}, mlvm.Word(0x3fc0_000)},
		{5, mlvm.IABC{B: 0x1ff}, mlvm.Word(0x0003_fe00)},
		{6, mlvm.IABC{C: 0x1ff}, mlvm.Word(0x0000_01ff)},
		{7, mlvm.IABC{Op: 0x3f, A: 0xff, B: 0x1ff, C: 0x1ff}, mlvm.Word(0xffff_ffff)},
	} {
		instruction := tc.input
		word := mlvm.EncodeABC(instruction)
		if tc.expect != word {
			t.Errorf("%d: want %x: got %x\n", tc.id, tc.expect, word)
		}
	}

}

func TestDecodeABC(t *testing.T) {
	type scenario struct {
		id     int
		input  mlvm.Word
		expect mlvm.IABC
	}

	for _, tc := range []scenario{
		{1, mlvm.Word(0), mlvm.IABC{}},
		{2, mlvm.Word(0x0400_0000), mlvm.IABC{Op: 0x01}},
		{3, mlvm.Word(0xfc00_0000), mlvm.IABC{Op: 0x3f}},
		{4, mlvm.Word(0x3fc0_000), mlvm.IABC{A: 0xff}},
		{5, mlvm.Word(0x0003_fe00), mlvm.IABC{B: 0x1ff}},
		{6, mlvm.Word(0x0000_01ff), mlvm.IABC{C: 0x1ff}},
		{7, mlvm.Word(0xffff_ffff), mlvm.IABC{Op: 0x3f, A: 0xff, B: 0x1ff, C: 0x1ff}},
	} {
		word := tc.input
		instruction := mlvm.DecodeABC(word)
		if tc.expect != instruction {
			t.Errorf("%d: want %s: got %s\n", tc.id, tc.expect, instruction)
		}
	}
}

func TestEncodeABx(t *testing.T) {
	type scenario struct {
		id     int
		input  mlvm.IABx
		expect mlvm.Word
	}
	for _, tc := range []scenario{
		{1, mlvm.IABx{}, mlvm.Word(0)},
		{2, mlvm.IABx{Op: 0x01}, mlvm.Word(0x0400_0000)},
		{3, mlvm.IABx{Op: 0x3f}, mlvm.Word(0xfc00_0000)},
		{4, mlvm.IABx{A: 0xff}, mlvm.Word(0x3fc0_000)},
		{5, mlvm.IABx{Bx: 0x01}, mlvm.Word(0x0000_0001)},
		{6, mlvm.IABx{Bx: 0x01ff}, mlvm.Word(0x0000_01ff)},
		{7, mlvm.IABx{Bx: 0x03_ffff}, mlvm.Word(0x0003_ffff)},
		{8, mlvm.IABx{Op: 0x3f, A: 0xff, Bx: 0x03_ffff}, mlvm.Word(0xffff_ffff)},
	} {
		instruction := tc.input
		word := mlvm.EncodeABx(instruction)
		if tc.expect != word {
			t.Errorf("%d: want %x: got %x\n", tc.id, tc.expect, word)
		}
	}
}

func TestDecodeABx(t *testing.T) {
	type scenario struct {
		id     int
		input  mlvm.Word
		expect mlvm.IABx
	}
	for _, tc := range []scenario{
		{1, mlvm.Word(0), mlvm.IABx{}},
		{2, mlvm.Word(0x0400_0000), mlvm.IABx{Op: 0x01}},
		{3, mlvm.Word(0xfc00_0000), mlvm.IABx{Op: 0x3f}},
		{4, mlvm.Word(0x3fc0_000), mlvm.IABx{A: 0xff}},
		{5, mlvm.Word(0x0000_0001), mlvm.IABx{Bx: 0x01}},
		{6, mlvm.Word(0x0000_01ff), mlvm.IABx{Bx: 0x01ff}},
		{7, mlvm.Word(0x0003_ffff), mlvm.IABx{Bx: 0x03_ffff}},
		{8, mlvm.Word(0xffff_ffff), mlvm.IABx{Op: 0x3f, A: 0xff, Bx: 0x03_ffff}},
	} {
		word := tc.input
		instruction := mlvm.DecodeABx(word)
		if tc.expect != instruction {
			t.Errorf("%d: want %s: got %s\n", tc.id, tc.expect, instruction)
		}
	}
}

func TestEncodeAsBx(t *testing.T) {
	type scenario struct {
		id     int
		input  mlvm.IAsBx
		expect mlvm.Word
	}
	for _, tc := range []scenario{
		{1, mlvm.IAsBx{}, mlvm.Word(0x001f_fff)},
		{2, mlvm.IAsBx{Op: 0x01}, mlvm.Word(0x0401_ffff)},
		{3, mlvm.IAsBx{Op: 0x3f}, mlvm.Word(0xfc01_ffff)},
		{4, mlvm.IAsBx{A: 0x01}, mlvm.Word(0x005f_fff)},
		{5, mlvm.IAsBx{A: 0xff}, mlvm.Word(0x03fd_ffff)},
		{6, mlvm.IAsBx{SBx: -131071}, mlvm.Word(0x0000_0000)},
		{7, mlvm.IAsBx{SBx: -131070}, mlvm.Word(0x0000_0001)},
		{8, mlvm.IAsBx{SBx: -131069}, mlvm.Word(0x0000_0002)},
		{9, mlvm.IAsBx{SBx: -2}, mlvm.Word(0x0001_fffd)},
		{10, mlvm.IAsBx{SBx: -1}, mlvm.Word(0x0001_fffe)},
		{11, mlvm.IAsBx{SBx: 0}, mlvm.Word(0x0001_ffff)},
		{11, mlvm.IAsBx{SBx: 1}, mlvm.Word(0x0002_0000)},
		{12, mlvm.IAsBx{SBx: 2}, mlvm.Word(0x0002_0001)},
		{13, mlvm.IAsBx{SBx: 131070}, mlvm.Word(0x0003_fffd)},
		{14, mlvm.IAsBx{SBx: 131071}, mlvm.Word(0x0003_fffe)},
		{15, mlvm.IAsBx{SBx: 131072}, mlvm.Word(0x0003_ffff)},
		{16, mlvm.IAsBx{Op: 0x3f, A: 0xff, SBx: -131071}, mlvm.Word(0xfffc_0000)},
		{17, mlvm.IAsBx{Op: 0x3f, A: 0xff, SBx: 131072}, mlvm.Word(0xffff_ffff)},
	} {
		instruction := tc.input
		word := mlvm.EncodeAsBx(instruction)
		if tc.expect != word {
			t.Errorf("%d: want %x: got %x\n", tc.id, tc.expect, word)
		}
	}
}

func TestDecodeAsBx(t *testing.T) {
	type scenario struct {
		id     int
		input  mlvm.Word
		expect mlvm.IAsBx
	}
	for _, tc := range []scenario{
		{1, mlvm.Word(0x001f_fff), mlvm.IAsBx{}},
		{2, mlvm.Word(0x0401_ffff), mlvm.IAsBx{Op: 0x01}},
		{3, mlvm.Word(0xfc01_ffff), mlvm.IAsBx{Op: 0x3f}},
		{4, mlvm.Word(0x005f_fff), mlvm.IAsBx{A: 0x01}},
		{5, mlvm.Word(0x03fd_ffff), mlvm.IAsBx{A: 0xff}},
		{6, mlvm.Word(0x0000_0000), mlvm.IAsBx{SBx: -131071}},
		{7, mlvm.Word(0x0000_0001), mlvm.IAsBx{SBx: -131070}},
		{8, mlvm.Word(0x0000_0002), mlvm.IAsBx{SBx: -131069}},
		{9, mlvm.Word(0x0001_fffd), mlvm.IAsBx{SBx: -2}},
		{10, mlvm.Word(0x0001_fffe), mlvm.IAsBx{SBx: -1}},
		{11, mlvm.Word(0x0001_ffff), mlvm.IAsBx{SBx: 0}},
		{11, mlvm.Word(0x0002_0000), mlvm.IAsBx{SBx: 1}},
		{12, mlvm.Word(0x0002_0001), mlvm.IAsBx{SBx: 2}},
		{13, mlvm.Word(0x0003_fffd), mlvm.IAsBx{SBx: 131070}},
		{14, mlvm.Word(0x0003_fffe), mlvm.IAsBx{SBx: 131071}},
		{15, mlvm.Word(0x0003_ffff), mlvm.IAsBx{SBx: 131072}},
		{16, mlvm.Word(0xfffc_0000), mlvm.IAsBx{Op: 0x3f, A: 0xff, SBx: -131071}},
		{17, mlvm.Word(0xffff_ffff), mlvm.IAsBx{Op: 0x3f, A: 0xff, SBx: 131072}},
	} {
		word := tc.input
		instruction := mlvm.DecodeAsBx(word)
		if tc.expect != instruction {
			t.Errorf("%d: want %s: got %s\n", tc.id, tc.expect, instruction)
		}
	}
}
