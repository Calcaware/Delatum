package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type parsedLine struct {
	opcode   string
	operands []string
	lineNum  int
}

type asmDef struct {
	opcode byte
	nops   int
}

var opcodeMap = map[string]asmDef{
	"set":   {opSet, 2},
	"mov":   {opMov, 2},
	"alloc": {opAlloc, 2},
	"add":   {opAdd, 3},
	"sub":   {opSub, 3},
	"mul":   {opMul, 3},
	"div":   {opDiv, 3},
	"mod":   {opMod, 3},
	"abs":   {opAbs, 2},
	"and":   {opAnd, 3},
	"or":    {opOr, 3},
	"xor":   {opXor, 3},
	"not":   {opNot, 2},
	"shl":   {opShl, 3},
	"ashr":  {opAshr, 3},
	"lshr":  {opLshr, 3},
	"jmp":   {opJmp, 1},
	"jeq":   {opJeq, 3},
	"jne":   {opJne, 3},
	"jlt":   {opJlt, 3},
	"jgt":   {opJgt, 3},
	"jle":   {opJle, 3},
	"jge":   {opJge, 3},
	"halt":  {opHalt, 1},
	"len":   {opLen, 1},
	"putc":  {opPutc, 2},
	"getc":  {opGetc, 2},
	"open":  {opOpen, 3},
	"close": {opClose, 1},
}

type operand struct {
	typ byte
	val int32
}

func Assemble(path string) ([]byte, error) {
	lines, err := preprocess(path, make(map[string]bool))
	if err != nil {
		return nil, err
	}

	type rawInst struct {
		opcode     byte
		nops       int
		operands   []operand
		labelRefs  []int
		labelNames []string
	}

	var insts []rawInst
	labels := make(map[string]int)
	offset := 0

	for _, ln := range lines {
		if ln.opcode == "label" && len(ln.operands) >= 1 {
			labels[ln.operands[0]] = offset
			continue
		}

		def, ok := opcodeMap[ln.opcode]
		if !ok {
			return nil, fmt.Errorf("line %d: unknown opcode %q", ln.lineNum, ln.opcode)
		}

		if len(ln.operands) != def.nops {
			return nil, fmt.Errorf("line %d: %s expects %d operands, got %d",
				ln.lineNum, ln.opcode, def.nops, len(ln.operands))
		}

		ri := rawInst{
			opcode: def.opcode,
			nops:   def.nops,
		}

		for _, opText := range ln.operands {
			op, isLabel := parseOperand(opText)
			ri.operands = append(ri.operands, op)
			if isLabel {
				ri.labelRefs = append(ri.labelRefs, len(ri.operands)-1)
				ri.labelNames = append(ri.labelNames, opText)
			}
		}

		offset += 1 + def.nops*5
		insts = append(insts, ri)
	}

	var buf []byte
	for _, inst := range insts {
		for i, refIdx := range inst.labelRefs {
			target, ok := labels[inst.labelNames[i]]
			if !ok {
				return nil, fmt.Errorf("undefined label %q", inst.labelNames[i])
			}
			inst.operands[refIdx] = operand{typ: operandTypeLiteral, val: int32(target)}
		}

		buf = append(buf, inst.opcode)
		for _, op := range inst.operands {
			buf = append(buf, encodeOperand(op.typ, op.val)...)
		}
	}

	return buf, nil
}

func preprocess(path string, seen map[string]bool) ([]parsedLine, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	absPath = filepath.Clean(absPath)

	if seen[absPath] {
		return nil, fmt.Errorf("circular include detected: %s", absPath)
	}
	seen[absPath] = true

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(absPath)
	rawLines := strings.Split(string(data), "\n")
	var lines []parsedLine
	var includes []parsedLine

	for lineNum, raw := range rawLines {
		raw = stripComment(raw)
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		tokens := tokenize(raw)
		if len(tokens) == 0 {
			continue
		}

		n := lineNum + 1

		if tokens[0] == "include" && len(tokens) >= 2 {
			incPath := strings.Trim(tokens[1], "\"")
			incLines, err := resolveInclude(incPath, dir, seen)
			if err != nil {
				return nil, fmt.Errorf("include at line %d: %w", n, err)
			}
			includes = append(includes, incLines...)
			continue
		}

		if tokens[0] == "set" && len(tokens) >= 3 && strings.HasPrefix(tokens[1], "\"") {
			str := tokens[1]
			str = str[1 : len(str)-1]
			destAddr, err := parseAddr(tokens[2])
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid set destination %q", n, tokens[2])
			}

			for _, ch := range []byte(str) {
				lines = append(lines, parsedLine{
					opcode:   "set",
					operands: []string{fmt.Sprintf("%d", int(ch)), fmt.Sprintf("@%d", destAddr)},
					lineNum:  n,
				})
				destAddr++
			}
			lines = append(lines, parsedLine{
				opcode:   "set",
				operands: []string{"0", fmt.Sprintf("@%d", destAddr)},
				lineNum:  n,
			})
			continue
		}

		lines = append(lines, parsedLine{
			opcode:   tokens[0],
			operands: tokens[1:],
			lineNum:  n,
		})
	}

	return append(lines, includes...), nil
}

func stripComment(line string) string {
	inString := false
	for i, ch := range line {
		if ch == '"' {
			inString = !inString
		}
		if ch == ';' && !inString {
			return line[:i]
		}
	}
	return line
}

func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inString := false

	for _, ch := range line {
		if inString {
			current.WriteRune(ch)
			if ch == '"' {
				tokens = append(tokens, current.String())
				current.Reset()
				inString = false
			}
			continue
		}
		if ch == '"' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			current.WriteRune(ch)
			inString = true
			continue
		}
		if unicode.IsSpace(ch) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func parseAddr(s string) (int64, error) {
	if strings.HasPrefix(s, "@") {
		return strconv.ParseInt(s[1:], 10, 64)
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseOperand(s string) (operand, bool) {
	if strings.HasPrefix(s, "@[") && strings.HasSuffix(s, "]") {
		inner := s[2 : len(s)-1]
		if strings.HasPrefix(inner, "@") {
			n, err := strconv.ParseInt(inner[1:], 10, 64)
			if err == nil {
				return operand{typ: operandTypeIndirect, val: int32(n)}, false
			}
		}
	}

	if strings.HasPrefix(s, "@") {
		n, err := strconv.ParseInt(s[1:], 10, 64)
		if err == nil {
			return operand{typ: operandTypeCell, val: int32(n)}, false
		}
	}

	if len(s) >= 3 && s[0] == '\'' && s[len(s)-1] == '\'' {
		inner := s[1 : len(s)-1]
		if len(inner) == 2 && inner[0] == '\\' {
			switch inner[1] {
			case 'n':
				return operand{typ: operandTypeLiteral, val: 10}, false
			case 't':
				return operand{typ: operandTypeLiteral, val: 9}, false
			case 'r':
				return operand{typ: operandTypeLiteral, val: 13}, false
			case '0':
				return operand{typ: operandTypeLiteral, val: 0}, false
			case '\\':
				return operand{typ: operandTypeLiteral, val: '\\'}, false
			case '\'':
				return operand{typ: operandTypeLiteral, val: '\''}, false
			}
		}
		if len(inner) >= 1 {
			return operand{typ: operandTypeLiteral, val: int32(inner[0])}, false
		}
	}

	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return operand{typ: operandTypeLiteral, val: int32(n)}, false
	}

	return operand{typ: operandTypeLiteral, val: 0}, true
}

func encodeOperand(typ byte, val int32) []byte {
	buf := make([]byte, 5)
	buf[0] = typ
	binary.LittleEndian.PutUint32(buf[1:], uint32(val))
	return buf
}

func resolveInclude(name, sourceDir string, seen map[string]bool) ([]parsedLine, error) {
	searchDirs := []string{sourceDir}

	cwd, _ := os.Getwd()
	if cwd != "" {
		searchDirs = append(searchDirs, filepath.Join(cwd, "twinc"))
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		searchDirs = append(searchDirs, filepath.Join(home, ".delatum", "twinc"))
	}

	searchDirs = append(searchDirs, "/usr/share/delatum/twinc")

	for _, dir := range searchDirs {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return preprocess(candidate, seen)
		}
	}

	return nil, fmt.Errorf("include file %q not found (searched: source dir, ./twinc/, ~/.delatum/twinc/, /usr/share/delatum/twinc/)", name)
}
