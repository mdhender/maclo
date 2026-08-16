// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package postfix

// precedence returns the precedence of the operator.
// Lower value means higher precedence.
func precedence(s string) int {
	switch s {
	case "*", "/":
		return 1
	case "+", "-":
		return 2
	default:
		// s is not a recognized operator
		return 10
	}
}

func FromInfix(expr []string) []string {
	// create a stack for storing operators and the converted postfix expression
	var s stack
	var postfix stack

	// process the infix expression from left to right
	for _, c := range expr {
		switch c {
		case "(": // left paren
			// push the paren on to the stack. it will serve as a sentinel
			// value for when we find the matching right paren.
			s.push(c)
		case ")": // right paren
			// pop from the stack until we find a left paren.
			for s.top() != "(" {
				// push on to the postfix stack
				postfix.push(s.top())
				s.pop()
			}
			// pop the left paren now
			s.pop()
		case "*", "/", "+", "-": // operand is an operator
			// remove operators from the stack with higher or equal precedence
			for !s.empty() && precedence(c) >= precedence(s.top()) {
				// push on to the postfix stack
				postfix.push(s.top())
				s.pop()
			}
			// push the operator on to the stack
			s.push(c)
		default: // operand is a variable or number
			// push on to the postfix stack
			postfix.push(c)
		}
	}

	// append any remaining operators in the stack to the postfix
	for !s.empty() {
		postfix.push(s.top())
		s.pop()
	}

	// return the converted expression
	return postfix.stack
}
