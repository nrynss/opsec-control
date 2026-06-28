# pkg/

Reusable, dependency-free helper libraries that are safe to import from anywhere
(unlike `internal/`, these could in principle be imported by external code).

Keep it small. Anything with domain meaning or cross-package shape belongs in
`internal/contracts`, not here. No state, no I/O singletons, no global mutable
state (SPEC §0.2 rule 4).
