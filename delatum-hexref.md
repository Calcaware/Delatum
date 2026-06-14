# Delatum Hex Editing Cheat Sheet
### Segment 1

---

## File Structure

```
[0-4]   54 57 4F 52 4D   TWORM magic bytes
[5]     01               Segment (version) number
[6...]  bytecode         Instructions, back to back
```

## Tape Layout at Startup

```
@0        = argc
@1...     = argv strings, null terminated
@n        = bytecode length in bytes
@n+1...   = bytecode
(free memory starts after bytecode)
```

---

## Operand Encoding

Every operand is exactly 5 bytes:

```
00 XX XX XX XX    literal integer
01 XX XX XX XX    cell address @n
02 XX XX XX XX    indirect @[@n]
```

`XX XX XX XX` is a 32-bit signed integer in platform byte order.

---

## Instruction Size

```
1 byte opcode + (5 bytes × operand count)
```

| Operands | Size |
|----------|------|
| 0 | 1 byte |
| 1 | 6 bytes |
| 2 | 11 bytes |
| 3 | 16 bytes |

---

## Opcode Table

### Memory `00-0F`

| Hex | Mnemonic | Operands | Notes |
|-----|----------|----------|-------|
| `01` | `set` | 2 | dst=op2, val=op1 |
| `02` | `mov` | 2 | src=op1, dst=op2 |
| `03` | `alloc` | 2 | size=op1, dst=op2 |

### Arithmetic `10-1F`

| Hex | Mnemonic | Operands | Notes |
|-----|----------|----------|-------|
| `10` | `add` | 3 | op3 = op1 + op2 |
| `11` | `sub` | 3 | op3 = op1 - op2 |
| `12` | `mul` | 3 | op3 = op1 * op2 |
| `13` | `div` | 3 | op3 = op1 / op2 |
| `14` | `mod` | 3 | op3 = op1 % op2 |
| `15` | `abs` | 2 | op2 = |op1| |

### Bitwise `20-2F`

| Hex | Mnemonic | Operands | Notes |
|-----|----------|----------|-------|
| `20` | `and` | 3 | op3 = op1 & op2 |
| `21` | `or` | 3 | op3 = op1 \| op2 |
| `22` | `xor` | 3 | op3 = op1 ^ op2 |
| `23` | `not` | 2 | op2 = ~op1 |
| `24` | `shl` | 3 | op3 = op1 << op2 |
| `25` | `ashr` | 3 | op3 = op1 >> op2 (arithmetic) |
| `26` | `lshr` | 3 | op3 = op1 >> op2 (logical) |

### Control Flow `30-3F`

| Hex | Mnemonic | Operands | Notes |
|-----|----------|----------|-------|
| `30` | `jmp` | 1 | jump to byte offset |
| `31` | `jeq` | 3 | jump if op1 == op2, target=op3 |
| `32` | `jne` | 3 | jump if op1 != op2, target=op3 |
| `33` | `jlt` | 3 | jump if op1 < op2, target=op3 |
| `34` | `jgt` | 3 | jump if op1 > op2, target=op3 |
| `35` | `jle` | 3 | jump if op1 <= op2, target=op3 |
| `36` | `jge` | 3 | jump if op1 >= op2, target=op3 |

### Halt & Special `40-4F`

| Hex | Mnemonic | Operands | Notes |
|-----|----------|----------|-------|
| `40` | `halt` | 1 | exit with code op1 |
| `4F` | `len` | 1 | dst=op1, stores total tape cell count |

### I/O `50-5F`

| Hex | Mnemonic | Operands | Notes |
|-----|----------|----------|-------|
| `50` | `putc` | 2 | fd=op1, char=op2, outputs op2 & 0xFF |
| `51` | `getc` | 2 | fd=op1, dst=op2, -1 on EOF |
| `52` | `open` | 3 | mode, filename (cell/indirect), handle_dest |
| `53` | `close` | 1 | fd=op1 |

---

## Reserved File Descriptors

```
00 00 00 00    stdin
00 00 00 01    stdout
00 00 00 02    stderr
```

---

## Quick Example - `halt 0`

```
40              opcode: halt
00 00 00 00 00  operand: literal 0
```

6 bytes total.

---

## Quick Example - `add @0 @1 @2`

```
10                    opcode: add
01 00 00 00 00        operand 1: cell 0
01 00 00 00 01        operand 2: cell 1
01 00 00 00 02        operand 3: cell 2
```

16 bytes total.

---

## Quick Example - `jne @[@0] 0 offset`

Jump back to byte offset 150 (0x96) if indirect cell 0 is not 0:

```
32                    opcode: jne
02 00 00 00 00        operand 1: indirect @[@0]
00 00 00 00 00        operand 2: literal 0
00 00 00 00 96        operand 3: literal 0x96 (byte offset 150)
```

16 bytes total.

---

## Tips for Hex Editing

- Always verify `TWORM` header at offset 0 before editing
- Count instruction sizes carefully - one wrong byte shifts every offset after it
- Jump targets are **byte offsets** from the start of the bytecode (after the 6 byte header)
- Recalculate all jump targets after inserting or removing instructions
- Write filename to tape with `set` before calling `open` - no inline strings in bytecode
- Little endian systems store `0x00000096` as `96 00 00 00`- know your platform
