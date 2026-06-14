# Delatum
### A Turing Mudpit - Segment 1

Delatum is a minimal assembly-style virtual machine language. It takes the core idea of brainfuck - a tape, simple operations, nothing else - and makes it actually usable without losing the spirit. You can write real programs. It just requires you to think carefully about memory.

The tape is the only storage. There is no heap, no call stack, no symbol table at runtime. Programs are assembled to bytecode, loaded onto the tape, and executed directly from it. The interpreter is trivial to implement in any language.

---

## Files

| Extension | Purpose |
|-----------|---------|
| `.tw` | Assembly source - write this |
| `.twi` | Include library - included by `.tw` files |
| `.twb` | Bytecode - assembled from `.tw`, run directly |

---

## The `tw` Tool

```bash
tw -i source.tw -o program.twb    # assemble to bytecode file
tw -r source.tw                    # assemble in memory and run immediately
./program.twb                      # run bytecode directly (after OS registration)
./source.tw                        # assemble and run (after OS registration)
```

`.twi` files cannot be run directly.

---

## The Tape

A flat array of signed integers. Platform native size, minimum 32-bit. The tape is the tapeworm. Each cell is a segment.

**Default size:** argc/argv area + bytecode length + (bytecode length × 2) working space

**Tape layout at startup:**
```
@0        = argc
@1...     = argv strings, null terminated, packed back to back
@n        = bytecode length in bytes
@n+1...   = bytecode
(free memory starts after bytecode - use alloc to grow the tape)
```

---

## Addressing

```
@5          ; value at cell 5
@[@0]       ; value at cell whose address is stored in cell 0
```

---

## Instruction Format

```
opcode arg1 arg2 arg3    ; comment
```

One instruction per line. Arguments are literals, `@n`, or `@[@n]`.

---

## Opcodes

### Memory
```
set 10 @0           ; @0 = 10
set 'A' @0          ; @0 = 65
set "hello" @0      ; write null terminated string to tape from @0
mov @0 @1           ; @1 = @0
alloc 256 @0        ; grow tape by 256 cells, @0 = start index
```

### Arithmetic
```
add @0 @1 @2        ; @2 = @0 + @1
sub @0 @1 @2
mul @0 @1 @2
div @0 @1 @2
mod @0 @1 @2
abs @0 @1           ; @1 = |@0|
```

### Bitwise
```
and @0 @1 @2
or  @0 @1 @2
xor @0 @1 @2
not @0 @1
shl @0 @1 @2        ; @2 = @0 << @1
ashr @0 @1 @2       ; arithmetic right shift (preserves sign)
lshr @0 @1 @2       ; logical right shift (shifts in zeros)
```

### Control Flow
```
label myloop        ; assembler only, resolved to byte offset
jmp myloop          ; unconditional jump
jmp @0              ; jump to byte offset stored in @0
jeq @0 @1 myloop    ; jump if @0 == @1
jne @0 @1 myloop    ; jump if @0 != @1
jlt @0 @1 myloop    ; jump if @0 < @1
jgt @0 @1 myloop    ; jump if @0 > @1
jle @0 @1 myloop    ; jump if @0 <= @1
jge @0 @1 myloop    ; jump if @0 >= @1
```

### Halt
```
halt                ; exit 0
halt @0             ; exit with code in @0
halt 1              ; exit with literal code
```

### I/O
```
putc 1 @0           ; write low byte of @0 to fd 1 (stdout)
getc 0 @0           ; read byte from fd 0 (stdin) into @0, -1 on EOF
open 0 @100 @1        ; open file named by string at @100 with mode 0, handle in @1
open @0 @1 @2         ; open file named by @1 with mode from @0, handle in @2
close @0            ; close handle
```

**Reserved file descriptors:** `0` stdin, `1` stdout, `2` stderr

---

## Routines

There is no `call` or `ret`. Store a return address manually and jump back to it:

```
set 0 @ret          ; @ret = byte offset to return to (assembler fills this)
jmp myroutine

label myroutine
; ... do work ...
jmp @ret            ; return
```

Nested calls require a manual call stack in tape cells. This is intentional.

---

## Includes

```
include "mylib.twi"
```

Assembler only. The contents of the included file are inlined at that point. Included files may include other files. Circular includes are a fatal error.

---

## Error Conditions

All errors halt the program immediately with a message:

- Integer overflow
- Division by zero
- `putc` out of valid character range
- Invalid file descriptor
- Tape address out of bounds
- Allocation failure
- Circular include (assembler)

---

## Running as an Executable

### Linux
```bash
echo ':twb:M::TWORM::/usr/bin/tw:' > /proc/sys/fs/binfmt_misc/register
echo ':tw:E::tw::/usr/bin/tw:' > /proc/sys/fs/binfmt_misc/register
chmod +x program.twb && ./program.twb
```

### macOS
Register `.tw` and `.twb` with Launch Services pointing to `tw`.

### Windows
Associate `.tw` and `.twb` with `tw.exe` in the registry.

---

## Examples

See `example.tw` for a complete program demonstrating string output, file I/O, CLI args, and manual routines.

See `example.twi` for a reusable include library with common string utilities.

---

## Bytecode Format

`.twb` files begin with a 6-byte header:
```
54 57 4F 52 4D    TWORM
01                Segment number
```

Each operand is 5 bytes: 1 type byte + 4 byte signed integer.
```
00 XX XX XX XX    literal
01 XX XX XX XX    cell address @n
02 XX XX XX XX    indirect @[@n]
```

See `delatum-spec.md` for the full bytecode specification and `delatum-hexref.md` for the hex editing cheat sheet.

---

## Philosophy

Delatum is not a Turing tarpit. It is a Turing mudpit. You can write real programs. It is just a bit grimy. That is the point.
