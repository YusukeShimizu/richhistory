# richhistory Concept

`richhistory` records shell commands with the context that built-in history does not keep: output previews, working directory changes, session identity, exit status, and duration.

The current version targets `zsh` and stores local NDJSON event files. It is meant to stay small, inspectable, and easy to use from the command line.

Because shell-level output capture can interfere with interactive terminal programs, `richhistory` prefers preserving interactive UX over forcing full capture. Known interactive commands fall back to metadata-only events unless the user explicitly overrides that behavior.

Optional helpers can build on top of that journal. AI-assisted workflows are examples of that extension point and live under `contrib/`.
