package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assembleAndRun(t *testing.T, src string, args ...string) (*VM, error) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.tw")
	if err := os.WriteFile(p, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	bc, err := Assemble(p)
	if err != nil {
		return nil, err
	}
	vm, err := NewVM(bc, args)
	if err != nil {
		return nil, err
	}
	err = vm.Run()
	return vm, err
}

func TestNewVMTapeLayout(t *testing.T) {
	bc := []byte{}
	vm, err := NewVM(bc, []string{"abc"})
	if err != nil {
		t.Fatal(err)
	}
	if vm.tape.cells[0] != 1 {
		t.Fatalf("argc: got %d, want 1", vm.tape.cells[0])
	}
	if vm.tape.cells[1] != 'a' || vm.tape.cells[2] != 'b' || vm.tape.cells[3] != 'c' || vm.tape.cells[4] != 0 {
		t.Fatalf("argv layout incorrect: %v", vm.tape.cells[1:5])
	}
}

func TestArithmetic(t *testing.T) {
	vm, err := assembleAndRun(t, `
set 5 @10
set 3 @11
add @10 @11 @12
sub @10 @11 @13
mul @10 @11 @14
halt 0
`)
	if err != nil {
		t.Fatal(err)
	}
	if vm.tape.cells[12] != 8 || vm.tape.cells[13] != 2 || vm.tape.cells[14] != 15 {
		t.Fatalf("arithmetic failed: %d %d %d", vm.tape.cells[12], vm.tape.cells[13], vm.tape.cells[14])
	}
}

func TestIndirectAddressing(t *testing.T) {
	vm, err := assembleAndRun(t, `
set 42 @10
set 10 @11
mov @[@11] @12
halt 0
`)
	if err != nil {
		t.Fatal(err)
	}
	if vm.tape.cells[12] != 42 {
		t.Fatalf("indirect addressing failed: got %d", vm.tape.cells[12])
	}
}

func TestControlFlowLoop(t *testing.T) {
	vm, err := assembleAndRun(t, `
set 0 @10
label loop
add @10 1 @10
jlt @10 5 loop
halt 0
`)
	if err != nil {
		t.Fatal(err)
	}
	if vm.tape.cells[10] != 5 {
		t.Fatalf("loop failed: got %d", vm.tape.cells[10])
	}
}

func TestAlloc(t *testing.T) {
	vm, err := assembleAndRun(t, `
alloc 4 @10
set 7 @[@10]
halt 0
`)
	if err != nil {
		t.Fatal(err)
	}
	if vm.tape.cells[vm.tape.cells[10]] != 7 {
		t.Fatalf("alloc failed: base=%d cell=%d", vm.tape.cells[10], vm.tape.cells[vm.tape.cells[10]])
	}
}

func TestLen(t *testing.T) {
	vm, err := assembleAndRun(t, `
len @10
halt 0
`)
	if err != nil {
		t.Fatal(err)
	}
	if vm.tape.cells[10] != 256 {
		t.Fatalf("len failed: got %d", vm.tape.cells[10])
	}
}

func TestHaltExitCode(t *testing.T) {
	vm, err := assembleAndRun(t, "halt 7\n")
	if err != nil {
		t.Fatal(err)
	}
	if vm.ExitCode != 7 {
		t.Fatalf("exit code: got %d, want 7", vm.ExitCode)
	}
}

func TestFileIO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	src := fmt.Sprintf(`
set "%s" @100
open 1 @100 @200
putc @200 65
close @200
set "%s" @100
open 0 @100 @201
getc @201 @202
close @201
halt 0
`, path, path)
	vm, err := assembleAndRun(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if vm.tape.cells[202] != 65 {
		t.Fatalf("file io failed: got %d", vm.tape.cells[202])
	}
}

func TestDivisionByZero(t *testing.T) {
	_, err := assembleAndRun(t, `
set 1 @10
set 0 @11
div @10 @11 @12
halt 0
`)
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("expected division by zero, got %v", err)
	}
}

func TestOutOfBoundsTape(t *testing.T) {
	_, err := assembleAndRun(t, `
set 999999 @999999
halt 0
`)
	if err == nil {
		t.Fatal("expected out-of-bounds error")
	}
}

func TestBareHalt(t *testing.T) {
	vm, err := assembleAndRun(t, "halt\n")
	if err != nil {
		t.Fatal(err)
	}
	if vm.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0", vm.ExitCode)
	}
}

func TestNaturalTermination(t *testing.T) {
	vm, err := assembleAndRun(t, `
set 42 @10
`)
	if err != nil {
		t.Fatal(err)
	}
	if vm.tape.cells[10] != 42 {
		t.Fatalf("natural termination failed: got %d", vm.tape.cells[10])
	}
}
