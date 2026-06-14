package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"
)

var nativeEndian binary.ByteOrder

func init() {
	var v uint32 = 0x01020304
	switch *(*byte)(unsafe.Pointer(&v)) {
	case 0x04:
		nativeEndian = binary.LittleEndian
	case 0x01:
		nativeEndian = binary.BigEndian
	}
}

const (
	opSet   byte = 0x01
	opMov   byte = 0x02
	opAlloc byte = 0x03

	opAdd byte = 0x10
	opSub byte = 0x11
	opMul byte = 0x12
	opDiv byte = 0x13
	opMod byte = 0x14
	opAbs byte = 0x15

	opAnd  byte = 0x20
	opOr   byte = 0x21
	opXor  byte = 0x22
	opNot  byte = 0x23
	opShl  byte = 0x24
	opAshr byte = 0x25
	opLshr byte = 0x26

	opJmp  byte = 0x30
	opJeq  byte = 0x31
	opJne  byte = 0x32
	opJlt  byte = 0x33
	opJgt  byte = 0x34
	opJle  byte = 0x35
	opJge  byte = 0x36

	opHalt byte = 0x40

	opLen byte = 0x4F

	opPutc byte = 0x50
	opGetc  byte = 0x51
	opOpen  byte = 0x52
	opClose byte = 0x53
)

const (
	operandTypeLiteral  byte = 0x00
	operandTypeCell     byte = 0x01
	operandTypeIndirect byte = 0x02
)

var operandCounts = [256]int{
	opSet:   2,
	opMov:   2,
	opAlloc: 2,
	opAdd:   3,
	opSub:   3,
	opMul:   3,
	opDiv:   3,
	opMod:   3,
	opAbs:   2,
	opAnd:   3,
	opOr:    3,
	opXor:   3,
	opNot:   2,
	opShl:   3,
	opAshr:  3,
	opLshr:  3,
	opJmp:   1,
	opJeq:   3,
	opJne:   3,
	opJlt:   3,
	opJgt:   3,
	opJle:   3,
	opJge:   3,
	opHalt:  1,
	opLen:   1,
	opPutc:  2,
	opGetc:  2,
	opOpen:  3,
	opClose: 1,
}

type VM struct {
	tape *Tape
	code []byte
	ip   int64
	fds  []*os.File
	halt bool
}

func NewVM(bytecode []byte, args []string) (*VM, error) {
	argvSize := 0
	for _, a := range args {
		argvSize += len(a) + 1
	}

	tapeSize := argvSize + 256
	if tapeSize < 256 {
		tapeSize = 256
	}

	tape := NewTape(tapeSize)

	tape.cells[0] = int64(len(args))

	off := int64(1)
	for _, a := range args {
		for _, ch := range []byte(a) {
			tape.cells[off] = int64(ch)
			off++
		}
		tape.cells[off] = 0
		off++
	}

	fds := make([]*os.File, 3)
	fds[0] = os.Stdin
	fds[1] = os.Stdout
	fds[2] = os.Stderr

	return &VM{
		tape: tape,
		code: bytecode,
		ip:   0,
		fds:  fds,
	}, nil
}

func (vm *VM) Run() error {
	for !vm.halt {
		if err := vm.step(); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) step() error {
	if vm.ip < 0 || vm.ip >= int64(len(vm.code)) {
		return fmt.Errorf("program counter out of bounds: %d", vm.ip)
	}
	opcode := vm.code[vm.ip]

	n := operandCounts[opcode]
	instSize := 1 + n*5

	switch opcode {
	case opSet:
		val, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("set: %w", err)
		}
		dst, err := vm.readAddr(vm.ip + 6)
		if err != nil {
			return fmt.Errorf("set: %w", err)
		}
		if err := vm.tape.Write(dst, val); err != nil {
			return fmt.Errorf("set: %w", err)
		}
	case opMov:
		src, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("mov: %w", err)
		}
		dst, err := vm.readAddr(vm.ip + 6)
		if err != nil {
			return fmt.Errorf("mov: %w", err)
		}
		if err := vm.tape.Write(dst, src); err != nil {
			return fmt.Errorf("mov: %w", err)
		}
	case opAlloc:
		size, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("alloc: %w", err)
		}
		dst, err := vm.readAddr(vm.ip + 6)
		if err != nil {
			return fmt.Errorf("alloc: %w", err)
		}
		start, err := vm.tape.Alloc(size)
		if err != nil {
			return fmt.Errorf("alloc: %w", err)
		}
		if err := vm.tape.Write(dst, start); err != nil {
			return fmt.Errorf("alloc: %w", err)
		}

	case opAdd:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		r, overflow := add64(a, b)
		if overflow {
			return fmt.Errorf("add: integer overflow")
		}
		vm.tape.Write(dst, r)

	case opSub:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		r, overflow := sub64(a, b)
		if overflow {
			return fmt.Errorf("sub: integer overflow")
		}
		vm.tape.Write(dst, r)

	case opMul:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		r, overflow := mul64(a, b)
		if overflow {
			return fmt.Errorf("mul: integer overflow")
		}
		vm.tape.Write(dst, r)

	case opDiv:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("div: division by zero")
		}
		vm.tape.Write(dst, a/b)

	case opMod:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("mod: division by zero")
		}
		vm.tape.Write(dst, a%b)

	case opAbs:
		v, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("abs: %w", err)
		}
		dst, err := vm.readAddr(vm.ip + 6)
		if err != nil {
			return fmt.Errorf("abs: %w", err)
		}
		if v < 0 {
			v = -v
		}
		vm.tape.Write(dst, v)

	case opAnd:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		vm.tape.Write(dst, a&b)

	case opOr:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		vm.tape.Write(dst, a|b)

	case opXor:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		vm.tape.Write(dst, a^b)

	case opNot:
		v, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("not: %w", err)
		}
		dst, err := vm.readAddr(vm.ip + 6)
		if err != nil {
			return fmt.Errorf("not: %w", err)
		}
		vm.tape.Write(dst, ^v)

	case opShl:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		vm.tape.Write(dst, a<<uint64(b))

	case opAshr:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		vm.tape.Write(dst, a>>uint64(b))

	case opLshr:
		a, b, dst, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		vm.tape.Write(dst, int64(uint64(a)>>uint64(b)))

	case opJmp:
		target, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("jmp: %w", err)
		}
		vm.ip = target
		return nil

	case opJeq:
		a, b, target, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		if a == b {
			vm.ip = target
			return nil
		}

	case opJne:
		a, b, target, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		if a != b {
			vm.ip = target
			return nil
		}

	case opJlt:
		a, b, target, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		if a < b {
			vm.ip = target
			return nil
		}

	case opJgt:
		a, b, target, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		if a > b {
			vm.ip = target
			return nil
		}

	case opJle:
		a, b, target, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		if a <= b {
			vm.ip = target
			return nil
		}

	case opJge:
		a, b, target, err := vm.read3(vm.ip)
		if err != nil {
			return err
		}
		if a >= b {
			vm.ip = target
			return nil
		}

	case opLen:
		dst, err := vm.readAddr(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("len: %w", err)
		}
		if err := vm.tape.Write(dst, vm.tape.Len()); err != nil {
			return fmt.Errorf("len: %w", err)
		}

	case opHalt:
		code, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("halt: %w", err)
		}
		vm.halt = true
		exitCode := int(code)
		if exitCode < 0 || exitCode > 255 {
			exitCode = 1
		}
		os.Exit(exitCode)
		return nil

	case opPutc:
		fd, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("putc: %w", err)
		}
		ch, err := vm.readValue(vm.ip + 6)
		if err != nil {
			return fmt.Errorf("putc: %w", err)
		}
		fdi := int(fd)
		if fdi < 0 || fdi >= len(vm.fds) || vm.fds[fdi] == nil {
			return fmt.Errorf("putc: invalid file descriptor %d", fdi)
		}
		if _, err := vm.fds[fdi].Write([]byte{byte(ch & 0xFF)}); err != nil {
			return fmt.Errorf("putc: %w", err)
		}

	case opGetc:
		fd, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("getc: %w", err)
		}
		dst, err := vm.readAddr(vm.ip + 6)
		if err != nil {
			return fmt.Errorf("getc: %w", err)
		}
		fdi := int(fd)
		if fdi < 0 || fdi >= len(vm.fds) || vm.fds[fdi] == nil {
			return fmt.Errorf("getc: invalid file descriptor %d", fdi)
		}
		buf := make([]byte, 1)
		_, err = vm.fds[fdi].Read(buf)
		if err != nil {
			vm.tape.Write(dst, -1)
		} else {
			vm.tape.Write(dst, int64(buf[0]))
		}

	case opOpen:
		mode, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}
		filenameAddr, err := vm.readValue(vm.ip + 6)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}
		handleDest, err := vm.readAddr(vm.ip + 11)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}

		var filename string
		ptr := filenameAddr
		for {
			ch, err := vm.tape.ReadByte(ptr)
			if err != nil {
				return fmt.Errorf("open: %w", err)
			}
			if ch == 0 {
				break
			}
			filename += string(ch)
			ptr++
		}

		var flags int
		switch mode {
		case 0:
			flags = os.O_RDONLY
		case 1:
			flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		case 2:
			flags = os.O_RDWR | os.O_CREATE
		default:
			return fmt.Errorf("open: invalid mode %d", mode)
		}

		f, err := os.OpenFile(filename, flags, 0644)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}

		fd := len(vm.fds)
		vm.fds = append(vm.fds, f)
		vm.tape.Write(handleDest, int64(fd))

	case opClose:
		fd, err := vm.readValue(vm.ip + 1)
		if err != nil {
			return fmt.Errorf("close: %w", err)
		}
		fdi := int(fd)
		if fdi < 0 || fdi >= len(vm.fds) || vm.fds[fdi] == nil {
			return fmt.Errorf("close: invalid file descriptor %d", fdi)
		}
		if fdi >= 3 {
			vm.fds[fdi].Close()
			vm.fds[fdi] = nil
		}

	default:
		return fmt.Errorf("unknown opcode 0x%02X at byte offset %d", opcode, vm.ip)
	}

	vm.ip += int64(instSize)
	return nil
}

func (vm *VM) rawOperand(addr int64) (typ byte, val int32, err error) {
	if addr < 0 || addr+5 > int64(len(vm.code)) {
		return 0, 0, fmt.Errorf("operand out of bounds at code offset %d", addr)
	}
	typ = vm.code[addr]
	val = int32(nativeEndian.Uint32(vm.code[addr+1 : addr+5]))
	return typ, val, nil
}

func (vm *VM) readValue(addr int64) (int64, error) {
	t, v, err := vm.rawOperand(addr)
	if err != nil {
		return 0, err
	}
	switch t {
	case operandTypeLiteral:
		return int64(v), nil
	case operandTypeCell:
		return vm.tape.Read(int64(v))
	case operandTypeIndirect:
		addr2, err := vm.tape.Read(int64(v))
		if err != nil {
			return 0, err
		}
		return vm.tape.Read(addr2)
	default:
		return 0, fmt.Errorf("unknown operand type 0x%02X", t)
	}
}

func (vm *VM) readAddr(addr int64) (int64, error) {
	t, v, err := vm.rawOperand(addr)
	if err != nil {
		return 0, err
	}
	switch t {
	case operandTypeLiteral:
		return int64(v), nil
	case operandTypeCell:
		return int64(v), nil
	case operandTypeIndirect:
		return vm.tape.Read(int64(v))
	default:
		return 0, fmt.Errorf("unknown operand type 0x%02X", t)
	}
}

func (vm *VM) read3(ip int64) (a, b, dst int64, err error) {
	a, err = vm.readValue(ip + 1)
	if err != nil {
		return 0, 0, 0, err
	}
	b, err = vm.readValue(ip + 6)
	if err != nil {
		return 0, 0, 0, err
	}
	dst, err = vm.readAddr(ip + 11)
	if err != nil {
		return 0, 0, 0, err
	}
	return a, b, dst, nil
}

func (vm *VM) readJump3(ip int64) (a, b, target int64, err error) {
	a, err = vm.readValue(ip + 1)
	if err != nil {
		return 0, 0, 0, err
	}
	b, err = vm.readValue(ip + 6)
	if err != nil {
		return 0, 0, 0, err
	}
	target, err = vm.readValue(ip + 11)
	if err != nil {
		return 0, 0, 0, err
	}
	return a, b, target, nil
}

func add64(a, b int64) (int64, bool) {
	r := a + b
	return r, (a > 0 && b > 0 && r <= 0) || (a < 0 && b < 0 && r >= 0)
}

func sub64(a, b int64) (int64, bool) {
	r := a - b
	return r, (a < 0 && b > 0 && r >= 0) || (a > 0 && b < 0 && r <= 0)
}

func mul64(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	r := a * b
	return r, r/a != b
}
