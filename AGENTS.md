# AGENTS

This repository is `richhistory`, a small Go CLI for richer shell journaling.

## Development Principles

1. Keep the CLI thin and push behavior into internal packages.
2. Preserve the Unix boundary: small interfaces, plain files, composable output.
3. Keep automatic shell integration explicit and inspectable.
4. Bound output growth with rotation and pruning.
5. Keep `README.md`, `concept.md`, and `spec.md` aligned with behavior changes.
6. Keep quality gates green with `just ci`.
