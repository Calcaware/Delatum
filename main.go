package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: delatum run <file.tw> [args...]")
			os.Exit(1)
		}
		runSource(os.Args[2], os.Args[3:])

	case "build":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: delatum build <file.tw>")
			os.Exit(1)
		}
		buildSource(os.Args[2])

	case "doc":
		cmdDoc()

	case "self-install":
		cmdSelfInstall()

	case "install":
		installHooks()

	case "uninstall":
		uninstallHooks()

	default:
		path := os.Args[1]

		if isBytecodeFile(path) {
			runBytecode(path, os.Args[2:])
		} else if strings.HasSuffix(path, ".tw") || strings.HasSuffix(path, ".twi") {
			runSource(path, os.Args[2:])
		} else if info, err := os.Stat(path); err == nil && !info.IsDir() {
			data, err := os.ReadFile(path)
			if err == nil && len(data) >= 5 && string(data[:5]) == "TWORM" {
				runBytecode(path, os.Args[2:])
			} else {
				runSource(path, os.Args[2:])
			}
		} else {
			fmt.Fprintf(os.Stderr, "delatum: unknown command %q\n", cmd)
			usage()
			os.Exit(1)
		}
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  delatum run <file.tw> [args...]     assemble and run")
	fmt.Fprintln(os.Stderr, "  delatum build <file.tw>             assemble to bytecode")
	fmt.Fprintln(os.Stderr, "  delatum doc [instruction]           show documentation")
	fmt.Fprintln(os.Stderr, "  delatum self-install                install to PATH")
	fmt.Fprintln(os.Stderr, "  delatum install                     register binfmt_misc hooks")
	fmt.Fprintln(os.Stderr, "  delatum uninstall                   remove binfmt_misc hooks")
	fmt.Fprintln(os.Stderr, "  delatum <file.twb> [args...]        run bytecode directly")
	fmt.Fprintln(os.Stderr, "  delatum <file.tw> [args...]         assemble and run")
}

func isBytecodeFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	header := make([]byte, 5)
	if _, err := f.Read(header); err != nil {
		return false
	}
	return string(header) == "TWORM"
}

func runSource(path string, args []string) {
	bytecode, err := Assemble(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allArgs := append([]string{path}, args...)

	vm, err := NewVM(bytecode, allArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := vm.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}

func buildSource(path string) {
	bytecode, err := Assemble(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	outName := path
	if strings.HasSuffix(outName, ".tw") {
		outName = outName[:len(outName)-3] + ".twb"
	} else {
		outName = outName + ".twb"
	}

	f, err := os.Create(outName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	header := []byte("TWORM\x01")
	if _, err := f.Write(header); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if _, err := f.Write(bytecode); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("built: %s\n", outName)
}

func runBytecode(path string, args []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(data) < 6 || string(data[:5]) != "TWORM" {
		fmt.Fprintf(os.Stderr, "error: invalid bytecode file (missing TWORM header)\n")
		os.Exit(1)
	}

	allArgs := append([]string{path}, args...)

	vm, err := NewVM(data[6:], allArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := vm.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}

func installHooks() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: cannot determine executable path: %v\n", err)
		fmt.Fprintln(os.Stderr, "install: register manually:")
		fmt.Fprintf(os.Stderr, "  echo ':delatum-tw:M::TWORM::%s:' > /proc/sys/fs/binfmt_misc/register\n", "/path/to/delatum")
		fmt.Fprintf(os.Stderr, "  echo ':delatum-tw-src:E::tw::%s:' > /proc/sys/fs/binfmt_misc/register\n", "/path/to/delatum")
		return
	}
	exe, _ = filepath.Abs(exe)

	fmt.Printf("installing binfmt_misc hooks for %s\n", exe)

	if err := os.WriteFile("/proc/sys/fs/binfmt_misc/register",
		[]byte(fmt.Sprintf(":delatum-tw:M::TWORM::%s:\n", exe)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		fmt.Fprintln(os.Stderr, "hint: run as root or try --user mode")
	} else {
		fmt.Println("  registered: .twb → delatum (by TWORM magic)")
	}

	if err := os.WriteFile("/proc/sys/fs/binfmt_misc/register",
		[]byte(fmt.Sprintf(":delatum-tw-src:E::tw::%s:\n", exe)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
	} else {
		fmt.Println("  registered: .tw → delatum (by extension)")
	}

	bindir := "/etc/binfmt.d"
	if _, err := os.Stat(bindir); err == nil {
		conf := fmt.Sprintf(":delatum-tw:M::TWORM::%s:\n:delatum-tw-src:E::tw::%s:\n", exe, exe)
		if err := os.WriteFile(bindir+"/delatum.conf", []byte(conf), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "install: warning: could not write %s/delatum.conf: %v\n", bindir, err)
		} else {
			fmt.Printf("  persistent config written to %s/delatum.conf\n", bindir)
		}
	}
}

func uninstallHooks() {
	fmt.Println("uninstalling binfmt_misc hooks")

	bindir := "/etc/binfmt.d"
	confPath := bindir + "/delatum.conf"
	if _, err := os.Stat(confPath); err == nil {
		if err := os.Remove(confPath); err != nil {
			fmt.Fprintf(os.Stderr, "uninstall: could not remove %s: %v\n", confPath, err)
		} else {
			fmt.Printf("  removed %s\n", confPath)
		}
	}

	for _, name := range []string{"delatum-tw", "delatum-tw-src"} {
		path := "/proc/sys/fs/binfmt_misc/" + name
		if err := os.WriteFile(path, []byte("-1\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "  could not clear %s: %v\n", name, err)
		} else {
			fmt.Printf("  cleared %s\n", name)
		}
	}
}
