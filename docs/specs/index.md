# Specifications Index

> **Specs are pre-implementation artifacts** — what you *planned*. They capture conceptual design
> intent, high-level decisions, and the "why" behind features. Specs are human-curated,
> flat in structure, and deliberately size-controlled for quick reading.
>
> Contrast with [`docs/memory/index.md`](../memory/index.md): memory files are *post-implementation* —
> what actually happened. Memory files are the authoritative source of truth for system behavior,
> maintained by `/fab-continue` (hydrate).
>
> **Ownership**: Specs are written and maintained by humans. No automated tooling creates or
> enforces structure here — organize files however makes sense for your project.

| Spec | Description |
|------|-------------|
| [cli-surface](cli-surface.md) | Subcommands, args, flags, exit codes, stdout/stderr conventions, match resolution algorithm |
| [config-resolution](config-resolution.md) | `repos.yaml` search order, schema, `repo config init` flow, embedded starter content |
| [architecture](architecture.md) | `src/` layout, Go package responsibilities, wrapper boundaries, cross-platform strategy |
| [build-and-release](build-and-release.md) | Justfile, scripts/, goreleaser intent, GitHub Actions, homebrew-tap (release pipeline deferred to follow-up change) |
