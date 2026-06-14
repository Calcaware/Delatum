package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type instructionDoc struct {
	Syntax  string
	Summary string
	Example string
}

var instructions = map[string]instructionDoc{
	"set": {
		Syntax:  "set <val> <dst>",
		Summary: "Store a value into a tape cell. dst = val",
		Example: "set 42 @100      ; write literal 42 to cell 100\nset @200 @100    ; copy cell 200 into cell 100\nset @[@300] @100 ; copy cell pointed-to by cell 300 into cell 100",
	},
	"mov": {
		Syntax:  "mov <src> <dst>",
		Summary: "Copy a value from one cell to another. dst = src",
		Example: "mov @200 @300   ; copy value from cell 200 to cell 300\nmov 99 @400     ; store literal 99 in cell 400",
	},
	"alloc": {
		Syntax:  "alloc <size> <dst>",
		Summary: "Allocate cells on the tape. Grows the tape by size cells and stores the start address in dst.",
		Example: "alloc 4096 @201   ; allocate 4096 cells, store start at @201",
	},
	"add": {
		Syntax:  "add <a> <b> <dst>",
		Summary: "Addition. dst = a + b",
		Example: "add @100 @101 @200   ; @200 = @100 + @101\nadd @100 1 @200      ; @200 = @100 + 1\nadd 5 3 @200         ; @200 = 8",
	},
	"sub": {
		Syntax:  "sub <a> <b> <dst>",
		Summary: "Subtraction. dst = a - b",
		Example: "sub @100 @101 @200   ; @200 = @100 - @101",
	},
	"mul": {
		Syntax:  "mul <a> <b> <dst>",
		Summary: "Multiplication. dst = a * b",
		Example: "mul @100 10 @200    ; @200 = @100 * 10",
	},
	"div": {
		Syntax:  "div <a> <b> <dst>",
		Summary: "Integer division. dst = a / b. Fails on division by zero.",
		Example: "div @100 3 @200     ; @200 = @100 / 3",
	},
	"mod": {
		Syntax:  "mod <a> <b> <dst>",
		Summary: "Modulo (remainder). dst = a % b. Fails on division by zero.",
		Example: "mod @100 10 @200    ; @200 = @100 % 10",
	},
	"abs": {
		Syntax:  "abs <val> <dst>",
		Summary: "Absolute value. dst = |val|",
		Example: "abs -42 @200        ; @200 = 42",
	},
	"and": {
		Syntax:  "and <a> <b> <dst>",
		Summary: "Bitwise AND. dst = a & b",
		Example: "and @100 0xFF @200  ; mask low byte",
	},
	"or": {
		Syntax:  "or <a> <b> <dst>",
		Summary: "Bitwise OR. dst = a | b",
		Example: "or @100 0x100 @200  ; set bit 8",
	},
	"xor": {
		Syntax:  "xor <a> <b> <dst>",
		Summary: "Bitwise XOR. dst = a ^ b",
		Example: "xor @100 @101 @200  ; @200 = @100 ^ @101",
	},
	"not": {
		Syntax:  "not <val> <dst>",
		Summary: "Bitwise NOT (ones' complement). dst = ~val",
		Example: "not @100 @200       ; @200 = ~@100",
	},
	"shl": {
		Syntax:  "shl <a> <b> <dst>",
		Summary: "Logical shift left. dst = a << b",
		Example: "shl 1 8 @200        ; @200 = 256 (1 << 8)",
	},
	"ashr": {
		Syntax:  "ashr <a> <b> <dst>",
		Summary: "Arithmetic shift right (sign-extending). dst = a >> b",
		Example: "ashr -8 2 @200      ; @200 = -2 (sign extended)",
	},
	"lshr": {
		Syntax:  "lshr <a> <b> <dst>",
		Summary: "Logical shift right (zero-filling). dst = a >> b (unsigned)",
		Example: "lshr -8 2 @200      ; large positive result (no sign extension)",
	},
	"len": {
		Syntax:  "len <dst>",
		Summary: "Store the current tape length (total number of cells) into dst.",
		Example: "len @100            ; @100 = total tape cell count",
	},
	"jmp": {
		Syntax:  "jmp <label>",
		Summary: "Unconditional jump to a label.",
		Example: "label start         ; define label\n    jmp start        ; jump back to start",
	},
	"jeq": {
		Syntax:  "jeq <a> <b> <label>",
		Summary: "Jump if equal. Jumps to label if a == b.",
		Example: "jeq @100 0 done     ; if @100 == 0, jump to done",
	},
	"jne": {
		Syntax:  "jne <a> <b> <label>",
		Summary: "Jump if not equal. Jumps to label if a != b.",
		Example: "jne @100 0 not_zero ; if @100 != 0, jump to not_zero",
	},
	"jlt": {
		Syntax:  "jlt <a> <b> <label>",
		Summary: "Jump if less than (signed). Jumps to label if a < b.",
		Example: "jlt @100 @200 smaller ; if @100 < @200, jump to smaller",
	},
	"jgt": {
		Syntax:  "jgt <a> <b> <label>",
		Summary: "Jump if greater than (signed). Jumps to label if a > b.",
		Example: "jgt @100 0 positive ; if @100 > 0, jump to positive",
	},
	"jle": {
		Syntax:  "jle <a> <b> <label>",
		Summary: "Jump if less than or equal (signed). Jumps to label if a <= b.",
		Example: "jle @100 @200 lte   ; if @100 <= @200, jump to lte",
	},
	"jge": {
		Syntax:  "jge <a> <b> <label>",
		Summary: "Jump if greater than or equal (signed). Jumps to label if a >= b.",
		Example: "jge @100 0 ge       ; if @100 >= 0, jump to ge",
	},
	"halt": {
		Syntax:  "halt <code>",
		Summary: "Stop execution with an exit code (0-255). Equivalent to sys_exit.",
		Example: "halt 0              ; exit successfully\nhalt 1              ; exit with error",
	},
	"putc": {
		Syntax:  "putc <fd> <char>",
		Summary: "Write a single character to a file descriptor. fd=0 stdin, 1 stdout, 2 stderr, 3+ opened files.",
		Example: "putc 1 65          ; write 'A' to stdout\nputc 1 '\\n'        ; write newline to stdout",
	},
	"getc": {
		Syntax:  "getc <fd> <dst>",
		Summary: "Read a single character from a file descriptor. Stores -1 in dst on EOF.",
		Example: "getc 0 @100        ; read one byte from stdin into cell 100\njeq @100 -1 eof   ; check for end of file",
	},
	"open": {
		Syntax:  "open <mode> <addr> <handleDest>",
		Summary: "Open a file. mode: 0=read, 1=write(create/truncate), 2=read-write(create). addr points to a null-terminated filename string on the tape. Stores the file descriptor handle in handleDest.",
		Example: "open 0 1 @200      ; open file named by string at cell 1 for reading\n                      ; (argv[0] is at cell 1)\n                      ; store fd handle in cell 200",
	},
	"close": {
		Syntax:  "close <fd>",
		Summary: "Close a file descriptor. No-op for stdin/stdout/stderr (0-2).",
		Example: "close @200         ; close the file handle stored in cell 200",
	},
}

var directives = map[string]instructionDoc{
	"include": {
		Syntax:  "include \"<file>\"",
		Summary: "Include another source file. Searched in: source dir, ./twinc/, ~/.delatum/twinc/, /usr/share/delatum/twinc/",
		Example: "include \"example.twi\"  ; include library routines\ninclude \"utils/str.twi\"   ; include from subdirectory",
	},
	"label": {
		Syntax:  "label <name>",
		Summary: "Define a label at the current position in the bytecode. Labels are used as jump targets.",
		Example: "label loop\n    add @100 1 @100\n    jlt @100 10 loop\nlabel done\n    halt 0",
	},
}

func cmdDoc() {
	if len(os.Args) >= 3 {
		docCommand(os.Args[2])
		return
	}
	docAll()
}

func docAll() {
	names := make([]string, 0, len(instructions))
	for n := range instructions {
		names = append(names, n)
	}
	sort.Strings(names)

	var maxLen int
	for _, n := range names {
		s := instructions[n].Syntax
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}

	fmt.Println("DELATUM INSTRUCTION SET")
	fmt.Println()

	for _, n := range names {
		d := instructions[n]
		fmt.Printf("  %-*s  %s\n", maxLen, d.Syntax, d.Summary)
	}

	fmt.Println()
	fmt.Println("DIRECTIVES")
	fmt.Println()
	for n, d := range directives {
		fmt.Printf("  %-*s  %s\n", maxLen, d.Syntax, d.Summary)
		_ = n
	}

	fmt.Println()
	fmt.Println("OPERAND FORMS")
	fmt.Println()
	fmt.Println("  <val> can be:")
	fmt.Println("    123          literal integer")
	fmt.Println("    @123         value stored at tape cell 123")
	fmt.Println("    @[@123]      value at the address stored in cell 123 (indirect)")
	fmt.Println("    'c'          character literal")
	fmt.Println("    '\\n'         escaped character (\\n, \\t, \\r, \\\\, \\', \\0)")
	fmt.Println()
	fmt.Println("  <dst> can be:")
	fmt.Println("    @123         tape cell 123")
	fmt.Println("    @[@123]      cell whose address is stored in cell 123")
	fmt.Println()
	fmt.Println("  <label> can be any name defined with: label <name>")
	fmt.Println()
	fmt.Println("BUILT-IN REGISTERS")
	fmt.Println()
	fmt.Println("  @0             argument count (argc)")
	fmt.Println("  @1+            argument strings (argv), each null-terminated")
	fmt.Println("  @253           return address (callee should jump here)")
	fmt.Println("  @250-@252      caller/callee scratch registers")
	fmt.Println("  @255           general-purpose (used by example programs)")
	fmt.Println()
	fmt.Println("For detailed help on a specific instruction:")
	fmt.Println("  delatum doc <instruction>")
}

func docCommand(name string) {
	if d, ok := instructions[name]; ok {
		fmt.Printf("%s\n\n", strings.ToUpper(name))
		fmt.Printf("Syntax:   %s\n", d.Syntax)
		fmt.Printf("Summary:  %s\n", d.Summary)
		fmt.Println()
		fmt.Println("Example:")
		for _, line := range strings.Split(d.Example, "\n") {
			fmt.Printf("  %s\n", line)
		}
		return
	}
	if d, ok := directives[name]; ok {
		fmt.Printf("%s\n\n", strings.ToUpper(name))
		fmt.Printf("Syntax:   %s\n", d.Syntax)
		fmt.Printf("Summary:  %s\n", d.Summary)
		fmt.Println()
		fmt.Println("Example:")
		for _, line := range strings.Split(d.Example, "\n") {
			fmt.Printf("  %s\n", line)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "doc: unknown instruction %q\n", name)
	os.Exit(1)
}

func cmdSelfInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "self-install: cannot determine executable path: %v\n", err)
		os.Exit(1)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "self-install: cannot resolve absolute path: %v\n", err)
		os.Exit(1)
	}

	installed := false
	installDir := findInstallDir()
	target := filepath.Join(installDir, "delatum")

	if exe != target {
		if err := copyFile(exe, target); err != nil {
			fmt.Fprintf(os.Stderr, "self-install: %v\n", err)
			fmt.Fprintf(os.Stderr, "try: sudo cp %s /usr/local/bin/delatum\n", exe)
		} else {
			fmt.Printf("installed to %s\n", target)
			installed = true
		}
	} else {
		fmt.Printf("already installed at %s\n", target)
	}

	switch runtime.GOOS {
	case "linux":
		registerLinux(target)
	case "darwin":
		registerMacOS(target)
	case "windows":
		registerWindows(target)
	default:
		fmt.Printf("unknown platform %s; no file associations registered\n", runtime.GOOS)
	}

	if installed {
		fmt.Println("restart your file manager or log out/in for file associations to take effect")
	}
}

func registerLinux(exe string) {
	fmt.Println("registering .tw / .twb file associations for Linux...")

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}

	appsDir := filepath.Join(dataHome, "applications")
	mimeDir := filepath.Join(dataHome, "mime", "packages")
	os.MkdirAll(appsDir, 0755)
	os.MkdirAll(mimeDir, 0755)

	desktopFile := filepath.Join(appsDir, "delatum.desktop")
	desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Delatum
Comment=Delatum virtual machine
Exec="%s" run "%%f"
MimeType=text/x-delatum;application/x-delatum-bytecode;
Terminal=true
Categories=Development;
`, exe)
	if err := os.WriteFile(desktopFile, []byte(desktopContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not write %s: %v\n", desktopFile, err)
	} else {
		fmt.Printf("  wrote %s\n", desktopFile)
	}

	mimeFile := filepath.Join(mimeDir, "delatum.xml")
	mimeContent := `<?xml version="1.0"?>
<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">
  <mime-type type="text/x-delatum">
    <comment>Delatum source code</comment>
    <glob pattern="*.tw"/>
    <glob pattern="*.twi"/>
  </mime-type>
  <mime-type type="application/x-delatum-bytecode">
    <comment>Delatum bytecode</comment>
    <glob pattern="*.twb"/>
  </mime-type>
</mime-info>
`
	if err := os.WriteFile(mimeFile, []byte(mimeContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not write %s: %v\n", mimeFile, err)
	} else {
		fmt.Printf("  wrote %s\n", mimeFile)
	}

	exec.Command("update-mime-database", filepath.Join(dataHome, "mime")).Run()
	exec.Command("update-desktop-database", appsDir).Run()
	exec.Command("xdg-mime", "default", "delatum.desktop", "text/x-delatum").Run()
	exec.Command("xdg-mime", "default", "delatum.desktop", "application/x-delatum-bytecode").Run()
}

func registerMacOS(exe string) {
	fmt.Println("registering .tw / .twb file associations for macOS...")

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDocumentTypes</key>
  <array>
    <dict>
      <key>CFBundleTypeExtensions</key>
      <array>
        <string>tw</string>
        <string>twi</string>
      </array>
      <key>CFBundleTypeRole</key>
      <string>Editor</string>
    </dict>
    <dict>
      <key>CFBundleTypeExtensions</key>
      <array>
        <string>twb</string>
      </array>
      <key>CFBundleTypeRole</key>
      <string>Viewer</string>
    </dict>
  </array>
</dict>
</plist>
`)

	appSupport := filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "delatum")
	os.MkdirAll(appSupport, 0755)
	plistPath := filepath.Join(appSupport, "DelatumFileAssoc.plist")
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not write %s: %v\n", plistPath, err)
	} else {
		fmt.Printf("  wrote %s\n", plistPath)
	}

	out, err := exec.Command("duti", "-v").Output()
	if err == nil && len(out) > 0 {
		dutiCmds := []string{
			fmt.Sprintf("duti -s com.example.delatum text/x-delatum all"),
			fmt.Sprintf("duti -s com.example.delatum application/x-delatum-bytecode all"),
		}
		for _, c := range dutiCmds {
			exec.Command("sh", "-c", c).Run()
		}
		fmt.Println("  registered via duti (install duti with: brew install duti)")
	} else {
		fmt.Println("  install duti for automatic registration: brew install duti")
		fmt.Printf("  or configure manually in Finder > Get Info on a .tw file\n")
	}
}

func registerWindows(exe string) {
	fmt.Println("registering .tw / .twb file associations for Windows...")

	cmds := []string{
		fmt.Sprintf(`assoc .tw=Delatum.Source`),
		fmt.Sprintf(`ftype Delatum.Source="%s" run "%%%%1"`, exe),
		fmt.Sprintf(`assoc .twi=Delatum.Source`),
		fmt.Sprintf(`assoc .twb=Delatum.Bytecode`),
		fmt.Sprintf(`ftype Delatum.Bytecode="%s" "%%%%1"`, exe),
	}

	for _, c := range cmds {
		cmd := exec.Command("cmd", "/c", c)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s: %v\n", c, err)
		} else {
			fmt.Printf("  %s\n", strings.TrimSpace(string(out)))
		}
	}
	fmt.Println("  file associations may require administrator privileges")
	fmt.Println("  or use: Settings > Apps > Default Apps > Choose by file type")
}

func findInstallDir() string {
	if runtime.GOOS == "windows" {
		windir := os.Getenv("WINDIR")
		if windir != "" {
			return filepath.Join(windir, "System32")
		}
		return "C:\\Windows\\System32"
	}

	candidates := []string{
		"/usr/local/bin",
		"/usr/bin",
		filepath.Join(os.Getenv("HOME"), ".local", "bin"),
		filepath.Join(os.Getenv("HOME"), "bin"),
	}

	for _, d := range candidates {
		if d == "" {
			continue
		}
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			testFile := filepath.Join(d, ".delatum_write_test")
			if err := os.WriteFile(testFile, []byte{}, 0644); err == nil {
				os.Remove(testFile)
				return d
			}
		}
	}

	return "/usr/local/bin"
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	if err := os.WriteFile(dst, data, 0755); err != nil {
		return fmt.Errorf("writing %s: %w", dst, err)
	}

	return nil
}
