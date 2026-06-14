package main

import (
	"fmt"
	"math"
)

type Tape struct {
	cells []int64
}

func NewTape(size int) *Tape {
	return &Tape{cells: make([]int64, size)}
}

func (t *Tape) Len() int64 {
	return int64(len(t.cells))
}

func (t *Tape) Read(addr int64) (int64, error) {
	if addr < 0 || addr >= int64(len(t.cells)) {
		return 0, fmt.Errorf("tape address out of bounds: %d", addr)
	}
	return t.cells[addr], nil
}

func (t *Tape) Write(addr int64, val int64) error {
	if addr < 0 || addr >= int64(len(t.cells)) {
		return fmt.Errorf("tape address out of bounds: %d", addr)
	}
	t.cells[addr] = val
	return nil
}

func (t *Tape) Alloc(size int64) (int64, error) {
	if size <= 0 {
		return 0, fmt.Errorf("alloc size must be positive: %d", size)
	}
	start := int64(len(t.cells))
	t.cells = append(t.cells, make([]int64, size)...)
	return start, nil
}

func (t *Tape) ReadByte(addr int64) (byte, error) {
	v, err := t.Read(addr)
	if err != nil {
		return 0, err
	}
	return byte(v & 0xFF), nil
}

func (t *Tape) ReadInt32(addr int64) (int32, error) {
	b0, err := t.ReadByte(addr)
	if err != nil {
		return 0, err
	}
	b1, err := t.ReadByte(addr + 1)
	if err != nil {
		return 0, err
	}
	b2, err := t.ReadByte(addr + 2)
	if err != nil {
		return 0, err
	}
	b3, err := t.ReadByte(addr + 3)
	if err != nil {
		return 0, err
	}
	return int32(nativeEndian.Uint32([]byte{b0, b1, b2, b3})), nil
}

func checkOverflow(val int64) error {
	if val > math.MaxInt32 || val < math.MinInt32 {
		return fmt.Errorf("integer overflow: %d exceeds cell size", val)
	}
	return nil
}
