module trunion.io/pawl

go 1.26

toolchain go1.26.7

// Versions
//
// `go 1.26` is the current major line (1.26.7 is the latest patch). Note that
// this directive is a *minimum language version*, not "build with the newest" —
// the code's real floor is 1.22, which is where `for i := range n` landed. It is
// set higher deliberately: nothing here needs to build on an old toolchain,
// because what reaches a client is the compiled binary, not this source.
//
// `toolchain go1.26.7` exists because the system Go is 1.26.0 and apt has no
// newer package. With GOTOOLCHAIN=auto (the default) the go command fetches and
// caches 1.26.7 on first build, so everyone gets the same patched compiler
// without a sudo install. Costs one ~20s download, once.
//
// Dependencies
//
// No require block, deliberately. The Python original carries two dependencies
// (pydantic, typer); this port carries none. Everything it needs — JSON, XML,
// SHA-256, subprocess, CLI flags — is in the standard library, and the one gap
// (TOML) is a ~130 line subset parser in internal/policy/toml.go.
//
// The consequence worth caring about is not elegance, it is C-6: no module
// resolution, no lockfile, no supply chain to audit, and the output is one
// static binary with no runtime.
