# TODO

Audit notes for the Delatum project. Items below are follow-ups found while reviewing the documentation and implementation.

## Documentation

- [x] Corrected the documented `tw` workflow to the actual `delatum build` / `delatum run` commands.
- [x] Corrected the default tape size and startup tape layout to match the implementation (argc/argv only; bytecode is not stored on the tape).
- [x] Removed `putc` out-of-range from the runtime error list because the implementation writes the low byte without validating.
- [x] Corrected `halt` examples to require an exit code; the implementation does not accept a bare `halt`.
- [x] Corrected the bytecode-spec include example from `.tw` to `.twi`.
- [ ] Decide whether the conceptual "bytecode lives on the tape" model should be restored in the implementation or removed from all docs.
- [ ] Add a short section documenting that reaching the end of bytecode is a runtime error in the current implementation.
- [ ] Add tests that exercise every example in `README.md`, `delatum-spec.md`, and `doc.go`.

## Implementation

- [ ] Rename `Tape.ReadByte` or adapt it to the `io.Reader` convention to clear `go vet`.
- [ ] Make endianness consistent: `Assemble` currently emits little-endian operands while `NewVM` detects host endianness.
- [ ] Decide whether `putc` should validate character range and update docs accordingly.
- [ ] Decide whether a bare `halt` and implicit halt at end-of-bytecode should be supported.
- [ ] Add Go tests for assembly, VM execution, file I/O, includes, and error paths.
- [ ] Add a build/test workflow and pin the Go version in `go.mod` if needed.
- [ ] Review the built-in "register" conventions (`@250`-`@255`) and ensure they are clearly documented as conventions only.

## Examples
- [ ] Add static analysis for unsafe self-modifying bytecode programs.

- [ ] Verify and refresh `example.tw`, `example.twi`, and `test_inc.tw` after documentation changes.
- [ ] Add a minimal "hello world" example that uses `halt`.