// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ast

import "fmt"

type Parameters []*Parameter

type Parameter struct {
	Line, Col int
	Kind      Kind
	Number    int
	Text      string
}

func (parms Parameters) String() string {
	if len(parms) == 0 {
		return ""
	}
	s, sep := "", ""
	for i, parm := range parms {
		switch parm.Kind {
		case Expression:
			if i > 0 && parms[i-1].Kind == Macro {
				s = s + parm.Text
			} else {
				s = s + sep + parm.Text
			}
		case Label:
			s = s + sep + parm.Text
		case Macro:
			s = s + sep + parm.Text
		case Number:
			s = s + sep + fmt.Sprintf("%d", parm.Number)
		case QuotedText:
			s = s + sep + fmt.Sprintf("'%s'", parm.Text)
		case Variable:
			s = s + sep + parm.Text
		default:
			panic("bad p")
		}
		sep = ","
	}
	return s
}
