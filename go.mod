module github.com/nrynss/opsec-control

// Pinned per SPEC §19.1: every Builder/CI uses the identical toolchain so
// builds and deterministic replay are reproducible across macOS/Windows/Linux.
go 1.24

toolchain go1.24.5
