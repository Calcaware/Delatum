package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
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

// TestExampleReadsArgumentFile runs example.tw with a file name as the first
// user argument and checks that it reads that file through argv[1].
func TestExampleReadsArgumentFile(t *testing.T) {
	bytecode, err := Assemble("example.tw")
	if err != nil {
		t.Fatalf("assemble example.tw: %v", err)
	}

	dir := t.TempDir()
	name := filepath.Join(dir, "data.txt")
	const content = "argument payload"
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	const program = "example.tw"
	vm, err := NewVM(bytecode, []string{program, name})
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := os.CreateTemp(dir, "stdout")
	if err != nil {
		t.Fatal(err)
	}
	vm.fds[1] = stdout

	if err := vm.Run(); err != nil {
		t.Fatalf("run example.tw: %v", err)
	}
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}

	if vm.ExitCode != 0 {
		t.Fatalf("exit code is %d, want 0", vm.ExitCode)
	}

	// argv[1] starts right after argv[0] and its null terminator.
	argStart := int64(1 + len(program) + 1)
	gotArg, err := readTapeString(vm.tape, argStart)
	if err != nil {
		t.Fatal(err)
	}
	if gotArg != name {
		t.Fatalf("argv[1] is %q, want %q", gotArg, name)
	}
	if got := vm.tape.cells[249]; got != argStart {
		t.Fatalf("first argument points at %d, want %d", got, argStart)
	}

	out, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Arguments: 2\n",
		"First argument: " + name + "\n",
		"Reading file: " + name + "\n",
		"File contents:\n" + content + "\n",
		"Done.\n",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
}

// readTapeString reads a null terminated string from the tape.
func readTapeString(tape *Tape, addr int64) (string, error) {
	var b strings.Builder
	for {
		ch, err := tape.ReadByteAt(addr)
		if err != nil {
			return "", err
		}
		if ch == 0 {
			return b.String(), nil
		}
		b.WriteByte(ch)
		addr++
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
