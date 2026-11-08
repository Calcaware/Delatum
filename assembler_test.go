package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSource(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAssembleBasicInstructionSize(t *testing.T) {
	dir := t.TempDir()
	p := writeSource(t, dir, "a.tw", "halt 0\n")
	bc, err := Assemble(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(bc) != 1+5 {
		t.Fatalf("unexpected bytecode length: %d", len(bc))
	}
	if bc[0] != opHalt {
		t.Fatalf("unexpected opcode: 0x%02x", bc[0])
	}
}

func TestAssembleStringExpansion(t *testing.T) {
	dir := t.TempDir()
	p := writeSource(t, dir, "a.tw", "set \"Hi\" @1\n")
	bc, err := Assemble(p)
	if err != nil {
		t.Fatal(err)
	}
	// two characters + null terminator, three set instructions
	if len(bc) != 3*(1+2*5) {
		t.Fatalf("unexpected bytecode length: %d", len(bc))
	}
}

func TestAssembleLabelResolution(t *testing.T) {
	dir := t.TempDir()
	p := writeSource(t, dir, "a.tw", "jmp end\nlabel end\nhalt 0\n")
	if _, err := Assemble(p); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleCircularInclude(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "a.tw", "include \"b.twi\"\nhalt 0\n")
	writeSource(t, dir, "b.twi", "include \"a.tw\"\n")
	_, err := Assemble(filepath.Join(dir, "a.tw"))
	if err == nil || !strings.Contains(err.Error(), "circular include") {
		t.Fatalf("expected circular include error, got %v", err)
	}
}

func TestAssembleUnknownOpcode(t *testing.T) {
	dir := t.TempDir()
	p := writeSource(t, dir, "a.tw", "frobnicate 1\n")
	_, err := Assemble(p)
	if err == nil {
		t.Fatal("expected error for unknown opcode")
	}
}

func TestAssembleIncludeAtDirective(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "lib.twi", "set 1 @10\nhalt 0\n")
	mainPath := writeSource(t, dir, "main.tw", "include \"lib.twi\"\nset 2 @10\nhalt 0\n")
	inlinePath := writeSource(t, dir, "inline.tw", "set 1 @10\nhalt 0\nset 2 @10\nhalt 0\n")

	got, err := Assemble(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Assemble(inlinePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("include was not inlined at the directive")
	}
}

func TestAssembleRepeatedInclude(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "lib.twi", "set 1 @10\n")
	mainPath := writeSource(t, dir, "main.tw", "include \"lib.twi\"\ninclude \"lib.twi\"\nhalt 0\n")

	bc, err := Assemble(mainPath)
	if err != nil {
		t.Fatalf("repeated include should assemble: %v", err)
	}
	want := 2*(1+2*5) + (1 + 5)
	if len(bc) != want {
		t.Fatalf("bytecode length is %d, want %d", len(bc), want)
	}
}
