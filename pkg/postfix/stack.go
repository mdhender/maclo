// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package postfix

type stack struct {
	stack []string
}

// empty returns true when the stack is empty
func (s *stack) empty() bool {
	return len(s.stack) == 0
}

// pop will panic on an empty stack
func (s *stack) pop() {
	s.stack = s.stack[:len(s.stack)-1]
}

func (s *stack) push(c string) {
	s.stack = append(s.stack, c)
}

func (s *stack) push_back(c string) {
	s.stack = append([]string{c}, s.stack...)
}

func (s *stack) reverse() {
	for i, j := 0, len(s.stack)-1; i < j; i, j = i+1, j-1 {
		s.stack[i], s.stack[j] = s.stack[j], s.stack[i]
	}
}

// top will panic on an empty stack
func (s *stack) top() string {
	return s.stack[len(s.stack)-1]
}
