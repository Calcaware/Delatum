package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// TestHelloWorldExample assembles and runs the minimal example and checks the
// greeting and the explicit halt.
func TestHelloWorldExample(t *testing.T) {
	bytecode, err := Assemble("hello.tw")
	if err != nil {
		t.Fatalf("assemble hello.tw: %v", err)
	}

	vm, err := NewVM(bytecode, []string{"hello.tw"})
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	vm.fds[1] = stdout

	if err := vm.Run(); err != nil {
		t.Fatalf("run hello.tw: %v", err)
	}
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "Hello, world!\n"; got != want {
		t.Fatalf("output is %q, want %q", got, want)
	}
	if vm.ExitCode != 0 {
		t.Fatalf("exit code is %d, want 0", vm.ExitCode)
	}
}

// TestCommittedExampleBytecode keeps example.twb in step with example.tw and
// example.twi. Rebuild it with "go run . build example.tw" after changing
// either source file.
func TestCommittedExampleBytecode(t *testing.T) {
	bytecode, err := Assemble("example.tw")
	if err != nil {
		t.Fatalf("assemble example.tw: %v", err)
	}
	if nativeEndian == binary.BigEndian {
		bytecode = docLittleEndian(bytecode)
	}

	want := append([]byte("TWORM\x01"), bytecode...)
	got, err := os.ReadFile("example.twb")
	if err != nil {
		t.Fatalf("read example.twb: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal(`example.twb is out of date; run "go run . build example.tw"`)
	}
}
