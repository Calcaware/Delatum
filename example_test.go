package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

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
