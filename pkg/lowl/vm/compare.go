// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import "fmt"

type CMPRSLT int

// enums for CMPAT
const (
	IS_LT CMPRSLT = -1 // register is less than value
	IS_EQ CMPRSLT = 0  // register is equal to value
	IS_GR CMPRSLT = 1  // register is greater than value
)

func (m *VM) compare(r, v int) {
	if r < v {
		m.Registers.Cmp = IS_LT
	} else if r == v {
		m.Registers.Cmp = IS_EQ
	} else {
		m.Registers.Cmp = IS_GR
	}
}

// String implements the Stringer interface.
func (r CMPRSLT) String() string {
	switch r {
	case IS_LT:
		return "<<"
	case IS_EQ:
		return "=="
	case IS_GR:
		return ">>"
	}
	panic(fmt.Sprintf("assert(cmprslt != %d)", r))
}
