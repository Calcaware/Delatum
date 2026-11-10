# TODO

Audit notes for the Delatum project. Items below are follow-ups found while reviewing the documentation and implementation.

## Documentation

- [x] Corrected the documented `tw` workflow to the actual `delatum build` / `delatum run` commands.
- [x] Corrected the default tape size and startup tape layout to match the implementation (argc/argv only; bytecode is not stored on the tape).
- [x] Removed `putc` out-of-range from the runtime error list because the implementation writes the low byte without validating.
- [x] Corrected `halt` examples to require an exit code; the implementation does not accept a bare `halt`.
- [x] Corrected the bytecode-spec include example from `.tw` to `.twi`.
- [x] Removed the "bytecode lives on the tape" model from the docs. The implementation keeps the bytecode in a separate code array, so the model was not restored.
- [x] Documented that reaching the end of the bytecode stops the program with exit code 0 instead of raising a runtime error.
- [x] Add tests that exercise every example in `README.md`, `delatum-spec.md`, and `doc.go`.
- [x] Mention the `@250`-`@255` scratch convention in `README.md` and `delatum-spec.md` as well, so it does not hide behind the `doc` command.
- [x] Documented that `argv[0]` is the program path and that the first user argument follows it.

## Implementation

- [x] Rename `Tape.ReadByte` to `Tape.ReadByteAt` to clear `go vet`.
- [x] Make operand endianness consistent with the host platform.
- [x] Decide whether `putc` should validate character range and update docs accordingly.
- [x] Decide whether a bare `halt` and implicit halt at end-of-bytecode should be supported.
- [x] Add Go tests for assembly, VM execution, file I/O, includes, and error paths.
- [x] Add a GitHub Actions workflow for build, vet, and test.
- [x] Review the built-in "register" conventions (`@250`-`@255`) and ensure they are clearly documented as conventions only.
- [x] Add tests for the command line paths (`build`, `run`, and running a `.twb` file) so the argument handling is covered as well.
- [x] Inline includes at the include directive instead of appending them to the end of the file. Repeated includes of the same library are now allowed while true cycles still fail.
- [x] Make conditional jumps read a cell or indirect target as a byte offset, matching `jmp`.
- [x] Detect `mul` overflow when the minimum signed value is the second operand.

## Examples

- [x] Add a cross platform endian test using golden bytecode. `TestDocumentedBytecode` compares the documented dump with the assembler output and normalises the operand byte order.
- [x] Decided against static analysis for self-modifying bytecode. The code array is separate from the tape and no instruction can write to it.

- [x] Verify and refresh `example.tw`, `example.twi`, and `test_inc.tw` after documentation changes. The stale `example.twb` was rebuilt from `example.tw`.
- [x] Keep the committed `example.twb` in sync with `example.tw` in a test, so the bytecode artifact cannot drift again. `TestCommittedExampleBytecode` assembles the source and compares the result with the committed file.
- [ ] Add Windows and macOS CI runners.
- [x] Add a minimal "hello world" example that uses `halt`. `hello.tw` prints a greeting and exits explicitly, and `TestHelloWorldExample` checks the output and exit code.
- [ ] Add a test that runs `example.tw` with a filename argument and checks that the named file is read from `argv[1]`.
- [ ] Run `hello.tw` through the built command line tool in a test so the explicit `halt 0` exit status is covered from the CLI as well.
