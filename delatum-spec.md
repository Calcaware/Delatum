# Delatum - Language Specification
### Segment 1

---

## Overview

Delatum is a minimal assembly-style virtual machine language. The tape is the only storage. Programs are assembled to bytecode, which the virtual machine executes from a separate code array.

- Tool: `delatum`
- Assembly extension: `.tw`
- Include extension: `.twi`
- Bytecode extension: `.twb`
- Magic bytes: `TWORM` (see Bytecode Format)

### The `delatum` Tool

One tool handles everything:

```
delatum build source.tw            ; assemble source to bytecode file
delatum run source.tw [args...]    ; assemble in memory and run immediately, no file written
./program.twb                      ; OS invokes delatum, verify header, run bytecode
./source.tw                        ; OS invokes delatum, assemble in memory and run
```

`.twi` files are include libraries. They are not normally entry points, but `delatum` will assemble and run one if asked. Only `.tw` and `.twb` are intended to be registered with the OS for execution.

The `TWORM` header exists only in `.twb` files. In-memory assembly skips the header entirely - the bytecode goes straight into the code array.

---

## The Tape

The tape is a flat array of signed integers - the only memory in the system.

- **Cell size:** 64-bit signed integer
- **Default size:** argc/argv area plus 256 cells of working space (see Implementation Notes)
- **Runtime allocation:** `alloc N @0` grows the tape by N cells, stores start index of new cells in @0, halts on failure
- **Overflow:** `add`, `sub`, and `mul` report a runtime error when the result does not fit in a cell
- **Endianness:** Matches host platform

Each cell is a **segment**. The tape is the **tapeworm**.

---

## Tape Layout at Startup

```
@0          = argc
@1...       = argv strings, null terminated, packed back to back
(free memory starts after the last argv string - use alloc to grow the tape)
```

Free memory starts after the argc/argv area in this implementation; `alloc` is the recommended way to reserve working space at runtime.

---

## Addressing

```
@5          ; value at cell 5
@[@0]       ; value at cell whose address is stored in cell 0
```

If cell 0 contains 10, `@[@0]` is the value at cell 10. Either form can be used as an argument to any instruction. Nesting beyond one level of indirection is undefined behavior.

**Scratch convention:** Cells `@250`-`@255` are conventionally used as scratch registers by example programs. `@253` is commonly used as a return address for manual routines, while `@250`-`@252` and `@255` serve as caller/callee scratch. These cells are not reserved by the VM and carry no special meaning at runtime.

---

## Assembly Language

### Instruction Format

```
opcode arg1 arg2 arg3    ; optional comment
```

One instruction per line. Arguments are literals, `@n`, or `@[@n]`. Comments begin with `;`.

### Literals

| Form | Meaning |
|------|---------|
| `10` | integer literal |
| `-5` | negative integer literal |
| `'A'` | character literal, expands to ASCII/Unicode value |
| `"hello"` | string literal, assembler expands to individual byte sets, null terminated |

### Includes

```
include "file.twi"
```

The assembler inlines the contents of `file.twi` at the point of the include directive. The interpreter never sees include directives - they are resolved entirely at assembly time.

Included files may include other files. The assembler tracks the current include stack and throws an error if a circular include is detected:

```
error: circular include detected
  main.tw
  -> math.tw
  -> utils.tw
  -> math.tw  <-- cycle
```

Labels from included files are resolved into the final bytecode offset space and are jumpable from any file in the program.

---

### Opcodes

#### Memory (`00-0F`)

```
set 10 @0           ; @0 = 10
set 'A' @0          ; @0 = 65
set "hello" @0      ; assembler expands to individual set instructions, null terminated
mov @0 @1           ; @1 = @0
alloc 256 @0        ; grow tape by 256 cells, @0 = start index of new cells
```

`set` with a string literal is assembler syntax only. The assembler expands it to one `set` per character plus a null terminator. No string type exists in bytecode.

#### Arithmetic (`10-1F`)

```
add @0 @1 @2        ; @2 = @0 + @1
sub @0 @1 @2        ; @2 = @0 - @1
mul @0 @1 @2        ; @2 = @0 * @1
div @0 @1 @2        ; @2 = @0 / @1
mod @0 @1 @2        ; @2 = @0 % @1
abs @0 @1           ; @1 = absolute value of @0
```

Overflow on any arithmetic operation is a runtime error.

#### Bitwise (`20-2F`)

```
and @0 @1 @2        ; @2 = @0 & @1
or  @0 @1 @2        ; @2 = @0 | @1
xor @0 @1 @2        ; @2 = @0 ^ @1
not @0 @1           ; @1 = ~@0
shl @0 @1 @2        ; @2 = @0 << @1
ashr @0 @1 @2       ; @2 = @0 >> @1  (arithmetic, preserves sign)
lshr @0 @1 @2       ; @2 = @0 >> @1  (logical, shifts in zeros)
```

#### Control Flow (`30-3F`)

```
label myloop        ; declare a jump target (assembler only, not in bytecode)
jmp myloop          ; unconditional jump
jmp @0              ; jump to line number stored in cell 0
jeq @0 @1 myloop    ; jump if @0 == @1
jne @0 @1 myloop    ; jump if @0 != @1
jlt @0 @1 myloop    ; jump if @0 < @1
jgt @0 @1 myloop    ; jump if @0 > @1
jle @0 @1 myloop    ; jump if @0 <= @1
jge @0 @1 myloop    ; jump if @0 >= @1
```

Labels are assembler-time only. The assembler resolves them to byte offsets in the bytecode. They do not exist at runtime.

`jmp @0` jumps to the byte offset stored in cell 0. This is how routines return - store the return offset in a cell before jumping, jump back to it at the end.

#### Halt & Special (`40-4F`)

```
halt                ; exit with code 0
halt 0              ; exit with code 0
halt @0             ; exit with code stored in @0
halt 1              ; exit with literal code 1

len @0              ; @0 = total number of cells on the tape
```

A program halts implicitly when execution reaches the end of its bytecode. An explicit `halt` may also be used.

#### I/O (`50-5F`)

```
putc 1 @0         ; write low byte of @0 to file descriptor 1
getc 0 @0         ; read one byte from file descriptor 0 into @0, -1 on EOF
open 0 @100 @0    ; open file with mode 0 (read), handle in @0
open @0 @1 @2     ; open file named by @1 with mode from @0, handle in @2
close @0          ; close file handle
```

**Reserved file descriptors:**
```
0 = stdin
1 = stdout
2 = stderr
```

`putc` writes the low 8 bits of its value (`value & 0xFF`). This implementation does not treat out-of-range values as an error.

`getc` returns -1 on EOF. A null byte (0) in a file is a valid byte, not EOF.

File cursor advances automatically on each `getc`/`putc`. The OS manages cursor position internally.

`open` takes three operands: mode, filename address, and handle destination.

**Modes:**
| Value | Meaning |
|-------|---------|
| 0 | Read (O_RDONLY) |
| 1 | Write (O_WRONLY) |
| 2 | Read/Write (O_RDWR) |

The filename operand is a cell address (or indirect) pointing to a null-terminated string on the tape. Write the filename to the tape with `set` before calling `open`. The handle is stored in the destination cell and used with `getc`, `putc`, and `close`.

File descriptors 0 (stdin), 1 (stdout), and 2 (stderr) are pre-opened by the runtime and are always available without calling `open`.

---

### Routines

There is no `call` or `ret`. Routines are a user convention:

```
; store return offset before jumping
set after @0        ; @0 = byte offset of the return point
jmp myroutine

label myroutine
; ... do work ...
jmp @0              ; jump back to stored return address

label after
```

The user is responsible for managing return addresses on the tape. Nested calls require the user to manage a manual call stack in tape cells.

---

## Error Conditions

The following are runtime errors. The interpreter halts with an error message identifying the failing instruction:

- Integer overflow on `add`, `sub`, and `mul`
- Division by zero
- Invalid file descriptor
- Tape address out of bounds
- Unknown opcode (corrupt bytecode)

Reaching the end of the bytecode is not an error. Execution stops there and the program exits with code 0.

---

## Example Programs

### Hello World

```
set "Hello, World!" @1  ; write string to tape starting at cell 1
set 1 @0                ; @0 = pointer, starts at 1

label loop
putc 1 @[@0]            ; print char at address stored in @0
add @0 1 @0             ; advance pointer
jne @[@0] 0 loop        ; loop until null terminator
```

### Read file to tape

```
set "file.txt" @100     ; write filename to tape starting at @100
open 0 @100 @0          ; open file with mode 0, handle in @0
set 10 @1               ; tape position to write into

label loop
getc @0 @[@1]           ; read byte into tape at position stored in @1
jeq @[@1] -1 done       ; stop on EOF
add @1 1 @1             ; advance tape position
jmp loop

label done
close @0
```

### Print tape contents

```
set 10 @2               ; pointer to start of data

label print
jeq @[@2] -1 done       ; stop on EOF marker
putc 1 @[@2]            ; print char
add @2 1 @2             ; advance pointer
jmp print

label done
```

### Pack RGB565 pixel

```
; @0=r, @1=g, @2=b, result in @3
shl @0 11 @3
shl @1 5  @4
or  @3 @4 @3
or  @3 @2 @3
```

### Write to Linux framebuffer

```
set "/dev/fb0" @100     ; write device path to tape
open 1 @100 @5          ; open framebuffer for writing, handle in @5
putc @5 @3              ; write packed pixel byte
close @5
```

### Manual routine with return address

```
set after @0               ; store return offset in @0
jmp myroutine               ; jump to routine

label myroutine
putc 1 65                   ; print 'A'
jmp @0                      ; return

label after                 ; execution resumes here after the routine
```

---

## Bytecode Format

### File Header

Every `.twb` file begins with a 6-byte header:

```
54 57 4F 52 4D    ; "TWORM" magic bytes
01                ; segment (version) number, currently 01
```

The interpreter rejects any file whose first 5 bytes are not `TWORM`.

### Operand Encoding

Every operand is exactly 5 bytes:

```
00 + [4 bytes]    ; literal integer
01 + [4 bytes]    ; cell address @n
02 + [4 bytes]    ; indirect @[@n]
```

The 4-byte value is a 32-bit signed integer in platform byte order.

### Instruction Encoding

Each instruction is 1 opcode byte followed by its operands. Every instruction has a fixed number of operands, all encoded as 5-byte operands. Instruction length is always known from the opcode.

```
opcode (1 byte) + N operands (5 bytes each)
```

| Operand count | Instruction size |
|---------------|-----------------|
| 0 | 1 byte |
| 1 | 6 bytes |
| 2 | 11 bytes |
| 3 | 16 bytes |

### Opcode Table

#### Memory (`00-0F`)

| Opcode | Mnemonic | Operands | Description |
|--------|----------|----------|-------------|
| `01` | `set` | 2 | set cell to value |
| `02` | `mov` | 2 | copy cell to cell |
| `03` | `alloc` | 2 | grow tape by N cells, store start index |

#### Arithmetic (`10-1F`)

| Opcode | Mnemonic | Operands | Description |
|--------|----------|----------|-------------|
| `10` | `add` | 3 | add |
| `11` | `sub` | 3 | subtract |
| `12` | `mul` | 3 | multiply |
| `13` | `div` | 3 | divide |
| `14` | `mod` | 3 | modulo |
| `15` | `abs` | 2 | absolute value |

#### Bitwise (`20-2F`)

| Opcode | Mnemonic | Operands | Description |
|--------|----------|----------|-------------|
| `20` | `and` | 3 | bitwise AND |
| `21` | `or` | 3 | bitwise OR |
| `22` | `xor` | 3 | bitwise XOR |
| `23` | `not` | 2 | bitwise NOT |
| `24` | `shl` | 3 | left shift |
| `25` | `ashr` | 3 | arithmetic right shift |
| `26` | `lshr` | 3 | logical right shift |

#### Control Flow (`30-3F`)

| Opcode | Mnemonic | Operands | Description |
|--------|----------|----------|-------------|
| `30` | `jmp` | 1 | unconditional jump to byte offset |
| `31` | `jeq` | 3 | jump if equal |
| `32` | `jne` | 3 | jump if not equal |
| `33` | `jlt` | 3 | jump if less than |
| `34` | `jgt` | 3 | jump if greater than |
| `35` | `jle` | 3 | jump if less than or equal |
| `36` | `jge` | 3 | jump if greater than or equal |

#### Halt & Special (`40-4F`)

| Opcode | Mnemonic | Operands | Description |
|--------|----------|----------|-------------|
| `40` | `halt` | 1 | halt with exit code |
| `4F` | `len` | 1 | store total tape cell count in dst |

#### I/O (`50-5F`)

| Opcode | Mnemonic | Operands | Description |
|--------|----------|----------|-------------|
| `50` | `putc` | 2 | write byte to fd |
| `51` | `getc` | 2 | read byte from fd |
| `52` | `open` | 3 | open file: mode, filename (cell/indirect), handle_dest |
| `53` | `close` | 1 | close file handle |

### Bytecode Example - Hello World

Assembly:
```
set "Hello, World!" @1
set 1 @0

label loop
putc 1 @[@0]
add @0 1 @0
jne @[@0] 0 loop
```

After string expansion and assembly (bytes in hex, comments added for clarity):

```
; Header
54 57 4F 52 4D 01

; set 72 @1  (H)
01  00 48000000  01 01000000
; set 101 @2 (e)
01  00 65000000  01 02000000
; set 108 @3 (l)
01  00 6C000000  01 03000000
; set 108 @4 (l)
01  00 6C000000  01 04000000
; set 111 @5 (o)
01  00 6F000000  01 05000000
; set 44  @6 (,)
01  00 2C000000  01 06000000
; set 32  @7 ( )
01  00 20000000  01 07000000
; set 87  @8 (W)
01  00 57000000  01 08000000
; set 111 @9 (o)
01  00 6F000000  01 09000000
; set 114 @10 (r)
01  00 72000000  01 0A000000
; set 108 @11 (l)
01  00 6C000000  01 0B000000
; set 100 @12 (d)
01  00 64000000  01 0C000000
; set 33  @13 (!)
01  00 21000000  01 0D000000
; set 0   @14 (null)
01  00 00000000  01 0E000000
; set 1 @0  (pointer = 1)
01  00 01000000  01 00000000

; loop: (byte offset 0xA5)
; putc 1 @[@0]
50  00 01000000  02 00000000
; add @0 1 @0
10  01 00000000  00 01000000  01 00000000
; jne @[@0] 0 loop
32  02 00000000  00 00000000  00 A5000000
```

The label `loop` resolved to byte offset `0xA5` (165) - the byte position of `putc` in the bytecode stream.

The operand values above are shown in little endian order, which is what a little endian host stores. A big endian host keeps the same values with the four value bytes reversed.

---

## Implementation Notes

- The interpreter is a fetch/decode/execute loop - read opcode byte, read N×5 operand bytes, execute
- No heap, no call stack, no symbol table required at runtime
- The assembler maintains a label table during assembly only - it is discarded after bytecode is emitted
- File handles map to OS file descriptors; cursor tracking is delegated to the OS
- Tape starts at a default size and can grow via `alloc`
- Bytecode lives in a separate code array, not on the tape - a program cannot modify its own code

---

## Running as a Native Executable

### Linux - binfmt_misc

Register `.tw` and `.twb` with the kernel:

```bash
echo ':twb:M::TWORM::/usr/bin/delatum:' > /proc/sys/fs/binfmt_misc/register
echo ':tw:E::tw::/usr/bin/delatum:' > /proc/sys/fs/binfmt_misc/register
```

After registration:

```bash
chmod +x program.twb
./program.twb        ; runs bytecode directly

chmod +x source.tw
./source.tw          ; assembles in memory and runs
```

To make registration persistent across reboots, add to a systemd service or `/etc/rc.local`.

### macOS - Launch Services

Register `.tw` and `.twb` as file types associated with `delatum`. Double clicking either in Finder runs the program. `.twi` files are not registered.

### Windows - File Association

```
HKEY_CLASSES_ROOT\.twb = DelatumBytecode
HKEY_CLASSES_ROOT\DelatumBytecode\shell\open\command = "C:\Program Files\delatum\delatum.exe" "%1"

HKEY_CLASSES_ROOT\.tw = DelatumSource
HKEY_CLASSES_ROOT\DelatumSource\shell\open\command = "C:\Program Files\delatum\delatum.exe" "%1"
```

The tool recognises `.tw` sources and `.twb` bytecode on its own, so no extra flag is needed. `.twi` files are not registered - they are include libraries only and cannot be executed directly.
