// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package assembler

import (
	"fmt"
	"github.com/maloquacious/ml_i/pkg/lowl/ast"
	"github.com/maloquacious/ml_i/pkg/postfix"
)

func evalMacro(macro string, expr *ast.Parameter, env map[string]int) (int, error) {
	switch expr.Kind {
	case ast.Expression:
		switch macro {
		case "OF":
			if value, err := ofMacro(parseExpr(expr.Text), env); err != nil {
				return 0, fmt.Errorf("macro: eval %w", err)
			} else {
				return value, nil
			}
		default:
			return 0, fmt.Errorf("macro: %s: not implemented", macro)
		}
	default:
		return 0, fmt.Errorf("macro: want expression: got %s", expr.Kind)
	}
}

// evaluate the of macro with the given arguments.
func ofMacro(expr []string, env map[string]int) (int, error) {
	// convert the expression to postfix and evaluate it
	return postfix.EvalPostfix(postfix.FromInfix(expr), env)
}

// parseExpr returns a slice from the expression.
func parseExpr(expr string) []string {
	var items []string
	var item string
	for ch, rest := expr[:1], expr[1:]; ch != ""; ch, rest = rest[:1], rest[1:] {
		switch ch {
		case "(", ")", "+", "*", "-":
			if item != "" {
				items = append(items, item)
			}
			items = append(items, ch)
			item = ""
		default:
			item = item + ch
		}

		if len(rest) == 0 {
			break
		}
	}
	return items
}
