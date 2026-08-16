// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package op

var stringToCode map[string]Code

func Lookup(s string) (Code, bool) {
	if stringToCode == nil {
		stringToCode = make(map[string]Code)
		stringToCode["AAL"] = AAL
		stringToCode["AAV"] = AAV
		stringToCode["ABV"] = ABV
		stringToCode["ALIGN"] = ALIGN
		stringToCode["ANDL"] = ANDL
		stringToCode["ANDV"] = ANDV
		stringToCode["BMOVE"] = BMOVE
		stringToCode["BSTK"] = BSTK
		stringToCode["BUMP"] = BUMP
		stringToCode["CAI"] = CAI
		stringToCode["CAL"] = CAL
		stringToCode["CAV"] = CAV
		stringToCode["CCI"] = CCI
		stringToCode["CCL"] = CCL
		stringToCode["CCN"] = CCN
		stringToCode["CFSTK"] = CFSTK
		stringToCode["CLEAR"] = CLEAR
		stringToCode["CON"] = CON
		stringToCode["CSS"] = CSS
		stringToCode["DCL"] = DCL
		stringToCode["EQU"] = EQU
		stringToCode["EXIT"] = EXIT
		stringToCode["EXITEQ"] = EXITEQ
		stringToCode["EXITGE"] = EXITGE
		stringToCode["EXITGR"] = EXITGR
		stringToCode["EXITLE"] = EXITLE
		stringToCode["EXITLT"] = EXITLT
		stringToCode["EXITND"] = EXITND
		stringToCode["EXITNE"] = EXITNE
		stringToCode["EXITPC"] = EXITPC
		stringToCode["FMOVE"] = FMOVE
		stringToCode["FSTK"] = FSTK
		stringToCode["GO"] = GO
		stringToCode["GOADD"] = GOADD
		stringToCode["GOEQ"] = GOEQ
		stringToCode["GOGE"] = GOGE
		stringToCode["GOGR"] = GOGR
		stringToCode["GOLE"] = GOLE
		stringToCode["GOLT"] = GOLT
		stringToCode["GOND"] = GOND
		stringToCode["GONE"] = GONE
		stringToCode["GOPC"] = GOPC
		stringToCode["GOSUB"] = GOSUB
		stringToCode["HASH"] = HASH
		stringToCode["IDENT"] = IDENT
		stringToCode["LAA"] = LAA
		stringToCode["LAI"] = LAI
		stringToCode["LAL"] = LAL
		stringToCode["LAM"] = LAM
		stringToCode["LAV"] = LAV
		stringToCode["LBV"] = LBV
		stringToCode["LCI"] = LCI
		stringToCode["LCM"] = LCM
		stringToCode["LCN"] = LCN
		stringToCode["LINKB"] = LINKB
		stringToCode["LINKR"] = LINKR
		stringToCode["MESS"] = MESS
		stringToCode["MULTL"] = MULTL
		stringToCode["NB"] = NB
		stringToCode["NCH"] = NCH
		stringToCode["NOOP"] = NOOP
		stringToCode["HALT"] = HALT
		stringToCode["ORL"] = ORL
		stringToCode["PRGEN"] = PRGEN
		stringToCode["PRGST"] = PRGST
		stringToCode["RL"] = RL
		stringToCode["SAL"] = SAL
		stringToCode["SAV"] = SAV
		stringToCode["SBL"] = SBL
		stringToCode["SBV"] = SBV
		stringToCode["STI"] = STI
		stringToCode["STR"] = STR
		stringToCode["STV"] = STV
		stringToCode["SUBR"] = SUBR
		stringToCode["THASH"] = THASH
		stringToCode["UNSTK"] = UNSTK
		stringToCode["WTHS"] = WTHS
	}
	code, ok := stringToCode[s]
	if !ok {
		code, ok = UNKNOWN, false
	}
	return code, ok
}
