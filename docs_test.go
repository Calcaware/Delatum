package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The documentation is the reference for the language, so every assembly
// example in it has to assemble, and every example that is a complete program
// has to run with the documented result. The tables below hold the expected
// behaviour of each example. An example that is not listed there fails the
// test, which keeps the docs and the implementation in step.

const (
	docModeAssemble = "assemble"
	docModeRun      = "run"
)

// docResult carries the state of a documented example after it ran.
type docResult struct {
	vm         *VM
	initialLen int64
}

// docExample describes what a documented example is expected to do.
type docExample struct {
	// mode is docModeAssemble for fragments that only have to assemble and
	// docModeRun for examples that are executed as a program.
	mode string
	// tail is appended to the example before it runs. Fragments that jump to a
	// label which is not part of the example use it to supply the missing
	// jump target.
	tail string
	// copyFiles are files from the repository that are placed next to the
	// example before it runs.
	copyFiles []string
	// files are written next to the example before it runs.
	files map[string]string
	// chdir runs the example with the temporary directory as working
	// directory.
	chdir bool
	// seed holds cell values that are written to the tape before the example
	// runs.
	seed map[int64]int64
	// stdin is the content of the file connected to descriptor 0.
	stdin string
	// wantOut is the exact output the example writes to stdout.
	wantOut string
	// wantExit is the exit code of the example.
	wantExit int
	// wantAsmErr is a part of the assembly error the example has to produce.
	// An empty string means the example has to assemble.
	wantAsmErr string
	// wantErr is a part of the runtime error the example has to produce. An
	// empty string means the example has to run without an error.
	wantErr string
	// wantCells holds cell values that are checked after the example ran.
	wantCells map[int64]int64
	// check is an optional extra check on the finished example.
	check func(t *testing.T, res docResult)
}

// readmeExamples holds the expectations for the examples in README.md. The
// key is the example without comments and with normalised spacing.
var readmeExamples = map[string]docExample{
	"set 10 @0\nset 'A' @0\nset \"hello\" @0\nmov @0 @1\nalloc 256 @0": {
		mode: docModeRun,
		check: func(t *testing.T, res docResult) {
			if got := res.vm.tape.cells[1]; got != 'h' {
				t.Errorf("cell 1 is %d, want %d", got, 'h')
			}
			if got := res.vm.tape.cells[0]; got != res.initialLen {
				t.Errorf("alloc returned %d, want %d", got, res.initialLen)
			}
			if got := res.vm.tape.Len(); got != res.initialLen+256 {
				t.Errorf("tape has %d cells, want %d", got, res.initialLen+256)
			}
		},
	},
	"add @0 @1 @2\nsub @0 @1 @2\nmul @0 @1 @2\ndiv @0 @1 @2\nmod @0 @1 @2\nabs @0 @1": {
		mode: docModeRun,
		seed: map[int64]int64{0: 3, 1: 2},
		wantCells: map[int64]int64{
			1: 3,
			2: 1,
		},
	},
	"and @0 @1 @2\nor @0 @1 @2\nxor @0 @1 @2\nnot @0 @1\nshl @0 @1 @2\nashr @0 @1 @2\nlshr @0 @1 @2": {
		mode: docModeRun,
		seed: map[int64]int64{0: 3, 1: 2},
		wantCells: map[int64]int64{
			1: -4,
			2: 0,
		},
	},
	"label myloop\njmp myloop\njmp @0\njeq @0 @1 myloop\njne @0 @1 myloop\njlt @0 @1 myloop\njgt @0 @1 myloop\njle @0 @1 myloop\njge @0 @1 myloop": {
		// The example jumps to itself, so it is only assembled.
		mode: docModeAssemble,
	},
	"halt\nhalt 0\nhalt @0\nhalt 1": {
		mode:     docModeRun,
		wantExit: 0,
	},
	"len @0": {
		mode: docModeRun,
		check: func(t *testing.T, res docResult) {
			if got := res.vm.tape.cells[0]; got != res.vm.tape.Len() {
				t.Errorf("len stored %d, want %d", got, res.vm.tape.Len())
			}
		},
	},
	"putc 1 @0\ngetc 0 @0\nopen 0 @100 @1\nopen @0 @1 @2\nclose @0": {
		// The lines are independent fragments. The file name in the tape is
		// empty, so the open fails while the example runs.
		mode:    docModeRun,
		wantErr: "open:",
	},
	"set after @0\njmp myroutine\nlabel myroutine\njmp @0\nlabel after": {
		mode: docModeRun,
	},
	"include \"mylib.twi\"": {
		mode:       docModeAssemble,
		wantAsmErr: "not found",
	},
}

// specExamples holds the expectations for the examples in delatum-spec.md.
var specExamples = map[string]docExample{
	"include \"file.twi\"": {
		mode:       docModeAssemble,
		wantAsmErr: "not found",
	},
	"set 10 @0\nset 'A' @0\nset \"hello\" @0\nmov @0 @1\nalloc 256 @0": {
		mode: docModeRun,
		check: func(t *testing.T, res docResult) {
			if got := res.vm.tape.cells[1]; got != 'h' {
				t.Errorf("cell 1 is %d, want %d", got, 'h')
			}
			if got := res.vm.tape.cells[0]; got != res.initialLen {
				t.Errorf("alloc returned %d, want %d", got, res.initialLen)
			}
			if got := res.vm.tape.Len(); got != res.initialLen+256 {
				t.Errorf("tape has %d cells, want %d", got, res.initialLen+256)
			}
		},
	},
	"add @0 @1 @2\nsub @0 @1 @2\nmul @0 @1 @2\ndiv @0 @1 @2\nmod @0 @1 @2\nabs @0 @1": {
		mode: docModeRun,
		seed: map[int64]int64{0: 3, 1: 2},
		wantCells: map[int64]int64{
			1: 3,
			2: 1,
		},
	},
	"and @0 @1 @2\nor @0 @1 @2\nxor @0 @1 @2\nnot @0 @1\nshl @0 @1 @2\nashr @0 @1 @2\nlshr @0 @1 @2": {
		mode: docModeRun,
		seed: map[int64]int64{0: 3, 1: 2},
		wantCells: map[int64]int64{
			1: -4,
			2: 0,
		},
	},
	"label myloop\njmp myloop\njmp @0\njeq @0 @1 myloop\njne @0 @1 myloop\njlt @0 @1 myloop\njgt @0 @1 myloop\njle @0 @1 myloop\njge @0 @1 myloop": {
		mode: docModeAssemble,
	},
	"halt\nhalt 0\nhalt @0\nhalt 1\nlen @0": {
		mode:     docModeRun,
		wantExit: 0,
	},
	"putc 1 @0\ngetc 0 @0\nopen 0 @100 @0\nopen @0 @1 @2\nclose @0": {
		// The file name in the tape is empty, so the open fails while the
		// example runs.
		mode:    docModeRun,
		wantErr: "open:",
	},
	"set after @0\njmp myroutine\nlabel myroutine\njmp @0\nlabel after": {
		mode: docModeRun,
	},
	"set \"Hello, World!\" @1\nset 1 @0\nlabel loop\nputc 1 @[@0]\nadd @0 1 @0\njne @[@0] 0 loop": {
		mode:    docModeRun,
		wantOut: "Hello, World!",
	},
	"set \"file.txt\" @100\nopen 0 @100 @0\nset 10 @1\nlabel loop\ngetc @0 @[@1]\njeq @[@1] -1 done\nadd @1 1 @1\njmp loop\nlabel done\nclose @0": {
		mode:  docModeRun,
		chdir: true,
		files: map[string]string{
			"file.txt": "abc",
		},
		wantCells: map[int64]int64{
			10: 'a',
			11: 'b',
			12: 'c',
			13: -1,
		},
	},
	"set 10 @2\nlabel print\njeq @[@2] -1 done\nputc 1 @[@2]\nadd @2 1 @2\njmp print\nlabel done": {
		mode:    docModeRun,
		wantOut: "abc",
		seed: map[int64]int64{
			10: 'a',
			11: 'b',
			12: 'c',
			13: -1,
		},
	},
	"shl @0 11 @3\nshl @1 5 @4\nor @3 @4 @3\nor @3 @2 @3": {
		mode: docModeRun,
		seed: map[int64]int64{
			0: 31,
			1: 63,
			2: 31,
		},
		wantCells: map[int64]int64{
			3: 0xFFFF,
			4: 63 << 5,
		},
	},
	"set \"/dev/fb0\" @100\nopen 1 @100 @5\nputc @5 @3\nclose @5": {
		// The example needs a Linux framebuffer device, so it is only
		// assembled.
		mode: docModeAssemble,
	},
	"set after @0\njmp myroutine\nlabel myroutine\nputc 1 65\njmp @0\nlabel after": {
		mode:    docModeRun,
		wantOut: "A",
	},
}

// docCommandExamples holds the expectations for the examples that the
// "delatum doc" command prints. The key is the instruction or directive name.
var docCommandExamples = map[string]docExample{
	"set": {
		mode:      docModeRun,
		seed:      map[int64]int64{130: 140, 140: 42},
		wantCells: map[int64]int64{100: 42},
	},
	"mov": {
		mode:      docModeRun,
		seed:      map[int64]int64{120: 7},
		wantCells: map[int64]int64{130: 7, 140: 99},
	},
	"alloc": {
		mode: docModeRun,
		check: func(t *testing.T, res docResult) {
			if got := res.vm.tape.cells[201]; got != res.initialLen {
				t.Errorf("alloc returned %d, want %d", got, res.initialLen)
			}
			if got := res.vm.tape.Len(); got != res.initialLen+4096 {
				t.Errorf("tape has %d cells, want %d", got, res.initialLen+4096)
			}
		},
	},
	"add": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 10, 101: 5},
		wantCells: map[int64]int64{200: 8},
	},
	"sub": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 10, 101: 4},
		wantCells: map[int64]int64{200: 6},
	},
	"mul": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 7},
		wantCells: map[int64]int64{200: 70},
	},
	"div": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 10},
		wantCells: map[int64]int64{200: 3},
	},
	"mod": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 27},
		wantCells: map[int64]int64{200: 7},
	},
	"abs": {
		mode:      docModeRun,
		wantCells: map[int64]int64{200: 42},
	},
	"and": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 0x1234},
		wantCells: map[int64]int64{200: 0x34},
	},
	"or": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 1},
		wantCells: map[int64]int64{200: 257},
	},
	"xor": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 0xF0, 101: 0x0F},
		wantCells: map[int64]int64{200: 0xFF},
	},
	"not": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 0},
		wantCells: map[int64]int64{200: -1},
	},
	"shl": {
		mode:      docModeRun,
		wantCells: map[int64]int64{200: 256},
	},
	"ashr": {
		mode:      docModeRun,
		wantCells: map[int64]int64{200: -2},
	},
	"lshr": {
		mode:      docModeRun,
		wantCells: map[int64]int64{200: 0x3FFFFFFFFFFFFFFE},
	},
	"len": {
		mode: docModeRun,
		check: func(t *testing.T, res docResult) {
			if got := res.vm.tape.cells[100]; got != res.vm.tape.Len() {
				t.Errorf("len stored %d, want %d", got, res.vm.tape.Len())
			}
		},
	},
	"jmp": {
		// The example jumps to itself, so it is only assembled.
		mode: docModeAssemble,
	},
	"jeq": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 0},
		tail:      "label done\nset 1 @150\nhalt 0",
		wantCells: map[int64]int64{150: 1},
	},
	"jne": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 1},
		tail:      "label not_zero\nset 1 @150\nhalt 0",
		wantCells: map[int64]int64{150: 1},
	},
	"jlt": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 1, 200: 2},
		tail:      "label smaller\nset 1 @150\nhalt 0",
		wantCells: map[int64]int64{150: 1},
	},
	"jgt": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 1},
		tail:      "label positive\nset 1 @150\nhalt 0",
		wantCells: map[int64]int64{150: 1},
	},
	"jle": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 2, 200: 2},
		tail:      "label lte\nset 1 @150\nhalt 0",
		wantCells: map[int64]int64{150: 1},
	},
	"jge": {
		mode:      docModeRun,
		seed:      map[int64]int64{100: 0},
		tail:      "label ge\nset 1 @150\nhalt 0",
		wantCells: map[int64]int64{150: 1},
	},
	"halt": {
		mode:     docModeRun,
		wantExit: 0,
	},
	"putc": {
		mode:    docModeRun,
		wantOut: "A\n",
	},
	"getc": {
		mode:  docModeRun,
		stdin: "",
		tail:  "label eof\nset 1 @150\nhalt 0",
		wantCells: map[int64]int64{
			100: -1,
			150: 1,
		},
	},
	"open": {
		mode:      docModeRun,
		wantCells: map[int64]int64{200: 3},
	},
	"close": {
		mode: docModeRun,
	},
	"include": {
		mode:      docModeAssemble,
		copyFiles: []string{"example.twi"},
		files: map[string]string{
			"utils/str.twi": "; placeholder\n",
		},
	},
	"label": {
		mode:      docModeRun,
		wantCells: map[int64]int64{100: 10},
	},
}

// TestMarkdownExamples assembles and runs every assembly example of the
// markdown documentation.
func TestMarkdownExamples(t *testing.T) {
	files := []struct {
		path  string
		table map[string]docExample
	}{
		{"README.md", readmeExamples},
		{"delatum-spec.md", specExamples},
	}

	for _, file := range files {
		t.Run(file.path, func(t *testing.T) {
			blocks := docCodeBlocks(t, file.path)
			used := map[string]bool{}
			index := 0

			for _, block := range blocks {
				if !docIsExample(block) {
					continue
				}
				key := docNormalize(block)
				example, ok := file.table[key]
				if !ok {
					t.Errorf("example is not covered by a test:\n%s", key)
					continue
				}
				used[key] = true
				name := fmt.Sprintf("%02d %s", index, docFirstLine(key))
				index++
				t.Run(name, func(t *testing.T) {
					runDocExample(t, block, example)
				})
			}

			for key := range file.table {
				if !used[key] {
					t.Errorf("test does not match any example in %s:\n%s", file.path, key)
				}
			}
		})
	}
}

// TestDocCommandExamples assembles and runs every example that the
// "delatum doc" command prints.
func TestDocCommandExamples(t *testing.T) {
	documented := map[string]instructionDoc{}
	for name, doc := range instructions {
		documented[name] = doc
	}
	for name, doc := range directives {
		documented[name] = doc
	}

	for name, doc := range documented {
		example, ok := docCommandExamples[name]
		if !ok {
			t.Errorf("example of %q is not covered by a test", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			runDocExample(t, doc.Example, example)
		})
	}

	for name := range docCommandExamples {
		if _, ok := documented[name]; !ok {
			t.Errorf("test %q does not match an instruction", name)
		}
	}
}

// TestDocumentedBytecode checks the byte dump of the hello world example in
// delatum-spec.md against the output of the assembler.
func TestDocumentedBytecode(t *testing.T) {
	var source, dump string
	for _, block := range docCodeBlocks(t, "delatum-spec.md") {
		lines := strings.Split(block, "\n")
		if len(lines) == 0 {
			continue
		}
		switch strings.TrimSpace(lines[0]) {
		case "set \"Hello, World!\" @1":
			source = block
		case "; Header":
			dump = block
		}
	}
	if source == "" || dump == "" {
		t.Fatal("the byte dump of the hello world example is missing")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "hello.tw")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	bytecode, err := Assemble(path)
	if err != nil {
		t.Fatal(err)
	}
	if nativeEndian == binary.BigEndian {
		bytecode = docLittleEndian(bytecode)
	}

	want := docHexBytes(t, dump)
	if len(want) < 6 {
		t.Fatal("the documented dump is too short")
	}
	if string(want[:5]) != "TWORM" {
		t.Errorf("documented header is %q, want %q", want[:5], "TWORM")
	}
	got := want[6:]
	if len(got) != len(bytecode) {
		t.Fatalf("documented dump has %d bytes, assembler produced %d", len(got), len(bytecode))
	}
	for i := range got {
		if got[i] != bytecode[i] {
			t.Fatalf("byte %d is 0x%02X, want 0x%02X", i, got[i], bytecode[i])
		}
	}
}

// docLittleEndian turns the operand values of a bytecode stream into little
// endian order, which is the order the documentation uses.
func docLittleEndian(bytecode []byte) []byte {
	out := append([]byte(nil), bytecode...)
	for i := 0; i < len(out); {
		count := operandCounts[out[i]]
		if count == 0 {
			break
		}
		for k := 0; k < count; k++ {
			base := i + 1 + k*5 + 1
			out[base], out[base+1], out[base+2], out[base+3] = out[base+3], out[base+2], out[base+1], out[base]
		}
		i += 1 + count*5
	}
	return out
}

// runDocExample assembles a documented example and checks that it behaves the
// way the documentation promises.
func runDocExample(t *testing.T, source string, example docExample) {
	t.Helper()

	dir := t.TempDir()
	for _, name := range example.copyFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		writeDocFile(t, dir, name, string(data))
	}
	for name, content := range example.files {
		writeDocFile(t, dir, name, content)
	}
	if example.chdir {
		t.Chdir(dir)
	}

	if example.tail != "" {
		source = source + "\n" + example.tail
	}
	path := filepath.Join(dir, "example.tw")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}

	bytecode, err := Assemble(path)
	if example.wantAsmErr != "" {
		if err == nil {
			t.Fatalf("assembly should fail with %q", example.wantAsmErr)
		}
		if !strings.Contains(err.Error(), example.wantAsmErr) {
			t.Fatalf("assembly error %q does not contain %q", err, example.wantAsmErr)
		}
		return
	}
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}
	if example.mode == docModeAssemble {
		return
	}

	vm, err := NewVM(bytecode, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	initialLen := vm.tape.Len()

	stdout := createDocFile(t, dir, "stdout")
	stderr := createDocFile(t, dir, "stderr")
	stdin := createDocFile(t, dir, "stdin")
	if _, err := stdin.WriteString(example.stdin); err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	vm.fds[0] = stdin
	vm.fds[1] = stdout
	vm.fds[2] = stderr

	for addr, value := range example.seed {
		if err := vm.tape.Write(addr, value); err != nil {
			t.Fatal(err)
		}
	}

	runErr := vm.Run()
	stdout.Close()
	stderr.Close()
	stdin.Close()
	for i := 3; i < len(vm.fds); i++ {
		if vm.fds[i] != nil {
			vm.fds[i].Close()
			vm.fds[i] = nil
		}
	}

	out, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}

	if example.wantErr != "" {
		if runErr == nil {
			t.Fatalf("expected a runtime error containing %q", example.wantErr)
		}
		if !strings.Contains(runErr.Error(), example.wantErr) {
			t.Fatalf("runtime error %q does not contain %q", runErr, example.wantErr)
		}
		return
	}
	if runErr != nil {
		t.Fatalf("runtime error: %v", runErr)
	}
	if vm.ExitCode != example.wantExit {
		t.Fatalf("exit code is %d, want %d", vm.ExitCode, example.wantExit)
	}
	if string(out) != example.wantOut {
		t.Fatalf("output is %q, want %q", out, example.wantOut)
	}
	for addr, want := range example.wantCells {
		got, err := vm.tape.Read(addr)
		if err != nil {
			t.Fatalf("read cell %d: %v", addr, err)
		}
		if got != want {
			t.Fatalf("cell %d is %d, want %d", addr, got, want)
		}
	}
	if example.check != nil {
		example.check(t, docResult{vm: vm, initialLen: initialLen})
	}
}

// docCodeBlocks returns the fenced code blocks of a markdown file.
func docCodeBlocks(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var blocks []string
	var current []string
	inside := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inside {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
				inside = false
			} else {
				inside = true
				current = nil
			}
			continue
		}
		if inside {
			current = append(current, strings.TrimRight(line, " \t"))
		}
	}
	return blocks
}

// docIsExample reports whether a documentation block holds Delatum
// instructions.
func docIsExample(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		fields := strings.Fields(stripComment(line))
		if len(fields) == 0 {
			continue
		}
		if _, ok := opcodeMap[fields[0]]; ok {
			return true
		}
		if fields[0] == "label" || fields[0] == "include" {
			return true
		}
	}
	return false
}

// docNormalize strips the comments and the redundant spacing from a
// documentation block so that examples can be matched reliably.
func docNormalize(block string) string {
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(stripComment(line))
		if line == "" {
			continue
		}
		lines = append(lines, strings.Join(tokenize(line), " "))
	}
	return strings.Join(lines, "\n")
}

// docFirstLine returns the first instruction of a normalised example.
func docFirstLine(key string) string {
	if i := strings.Index(key, "\n"); i >= 0 {
		return key[:i]
	}
	return key
}

// docHexBytes reads the hex dump of a documentation block.
func docHexBytes(t *testing.T, block string) []byte {
	t.Helper()

	var bytes []byte
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(stripComment(line))
		if line == "" {
			continue
		}
		for _, field := range strings.Fields(line) {
			value, err := strconv.ParseUint(field, 16, 32)
			if err != nil {
				t.Fatalf("%q is not a hex value", field)
			}
			for shift := (len(field) - 1) / 2; shift >= 0; shift-- {
				bytes = append(bytes, byte(value>>(8*shift)))
			}
		}
	}
	return bytes
}

// writeDocFile writes a file into the directory of a documented example.
func writeDocFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// createDocFile creates an empty file for the standard streams of a
// documented example.
func createDocFile(t *testing.T, dir, name string) *os.File {
	t.Helper()

	f, err := os.CreateTemp(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
