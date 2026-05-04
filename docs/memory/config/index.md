# config — memory

| File | Topic |
|---|---|
| [search-order](search-order.md) | `$HOP_CONFIG` → `$XDG_CONFIG_HOME/hop/hop.yaml` → `$HOME/.config/hop/hop.yaml`, hard-error semantics, no fallback to legacy `repos.yaml` paths |
| [yaml-schema](yaml-schema.md) | Grouped schema: `config:` + `repos:` named groups (flat list or `dir`/`urls` map); URL parsing; path resolution |
| [init-bootstrap](init-bootstrap.md) | `hop config init` write target, mode 0644, embedded grouped-form starter |
