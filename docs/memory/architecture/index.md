# architecture — memory

| File | Topic |
|---|---|
| [package-layout](package-layout.md) | `src/cmd/hop/` + `src/internal/<pkg>/`, cobra wiring, `-R` pre-Execute argv inspection, conventions |
| [wrapper-boundaries](wrapper-boundaries.md) | `internal/proc` security choke point (now exposes `RunCapture`), `internal/fzf` wrapper, `internal/yamled` comment-preserving YAML edits, `internal/scan` git invocation routing |
