// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package postfix

import (
	"fmt"
	"strconv"
)

func EvalPostfix(expr []string, env map[string]int) (int, error) {
	// replace variables with their values
	for i, arg := range expr {
		if arg == "+" || arg == "-" || arg == "*" || arg == "/" {
			// operators don't need replacing
			continue
		} else if _, err := strconv.Atoi(arg); err == nil {
			// valid integer, so keep it
			continue
		} else if n, ok := env[arg]; !ok {
			return 0, fmt.Errorf("unknown arg %q", arg)
		} else {
			expr[i] = fmt.Sprintf("%d", n)
		}
	}

	var a, b int
	var rs []int
	for _, elem := range expr {
		switch elem {
		case "*":
			b, rs = rs[len(rs)-1], rs[:len(rs)-1]
			a, rs = rs[len(rs)-1], rs[:len(rs)-1]
			rs = append(rs, a*b)
		case "/":
			b, rs = rs[len(rs)-1], rs[:len(rs)-1]
			a, rs = rs[len(rs)-1], rs[:len(rs)-1]
			rs = append(rs, a/b)
		case "+":
			b, rs = rs[len(rs)-1], rs[:len(rs)-1]
			a, rs = rs[len(rs)-1], rs[:len(rs)-1]
			rs = append(rs, a+b)
		case "-":
			b, rs = rs[len(rs)-1], rs[:len(rs)-1]
			a, rs = rs[len(rs)-1], rs[:len(rs)-1]
			rs = append(rs, a-b)
		default:
			if n, err := strconv.Atoi(elem); err != nil {
				return 0, fmt.Errorf("invalid number %q", elem)
			} else {
				rs = append(rs, n)
			}
		}
	}
	if len(rs) != 1 {
		return 0, fmt.Errorf("invalid expression")
	}
	return rs[0], nil
}
