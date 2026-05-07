# Memory Index

<!-- This index is maintained by /fab-continue (hydrate) when changes are completed. -->
<!-- Each domain gets a row linking to its memory files. -->

| Domain | Description | Memory Files |
|--------|-------------|------|
| cli | Command-line surface — subcommands, ad-hoc URL clone, match resolution, shell shim | [subcommands](cli/subcommands.md), [match-resolution](cli/match-resolution.md) |
| config | hop.yaml search order, grouped schema, bootstrap, and on-disk repo discovery via `hop config scan` | [search-order](config/search-order.md), [yaml-schema](config/yaml-schema.md), [init-bootstrap](config/init-bootstrap.md), [scan](config/scan.md) |
| architecture | Source tree layout, cobra wiring, `-R` argv inspection, wrapper boundaries | [package-layout](architecture/package-layout.md), [wrapper-boundaries](architecture/wrapper-boundaries.md) |
| build | Local build pipeline (justfile + scripts) and tag-driven release pipeline (GitHub Actions + homebrew-tap) | [local](build/local.md), [release-pipeline](build/release-pipeline.md) |
