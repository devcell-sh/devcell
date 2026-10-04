# Deprecating config keys, commands and flags

Never remove or rename a user-facing TOML key, command or flag in one step. Deprecate it first (old name keeps working and warns), remove it in a later release.

Every deprecation carries a message written as an instruction to the user: say what to use instead and show the new syntax with a short example value. Example: `use [secrets.onepassword] documents = ["prod-api-keys"] instead`.

## TOML keys

Every deprecated key is one entry in `cfg.Deprecations` (`internal/cfg/deprecations.go`). All four fields are required (`TestDeprecations_EveryEntryIsComplete` enforces it):

```go
{
	Path:        []string{"op"},          // old key path as decoded by BurntSushi/toml
	Name:        "[op]",                  // old key as the user writes it
	Replacement: "[secrets.onepassword]", // new key
	Message:     `use [secrets.onepassword] documents = ["prod-api-keys"] instead`,
},
```

Users see: `warning: <file>: <Name> is deprecated and will be removed in a future release: <Message>`.

To deprecate (rename) a key:

1. Add the new key. Keep the old struct field as the internal carrier consumers read; do not tag it `// Deprecated:` (staticcheck would flag every internal read).
2. In `LoadFile`, merge the new key's value into the old field (see `mergeCellPorts`, `mergeSecrets`), so both spellings work.
3. Add the `cfg.Deprecations` entry.
4. Switch `internal/scaffold/templates/*.tmpl`, `examples/*/.devcell.toml` and user-facing strings (error messages, comments) to the new key only.

Detection uses `toml.MetaData.IsDefined`, so only keys the user actually wrote warn. Warnings print once per `cell` invocation to stderr (`warnConfigDeprecations` in `cmd/root.go`), never stdout.

Aliases are not deprecations: an alias (e.g. `[secrets.op]` for `[secrets.onepassword]`) is a second struct field merged into the same carrier, with no table entry. Tag it `hm:"-"` so `options.nix` exposes only the canonical name.

A deprecated value form of a current key (e.g. `[llm] model = "ollama/qwen3:8b"`, now `provider` + `model`) cannot go in the key-based table, since the key itself is not deprecated. Normalize it where it is migrated and append a `DeprecatedUse` in `LoadFile` (see `llmModelPrefixDeprecation`).

`cell config migrate` rewrites deprecated keys in place (`cfg.MigrateTOML`, `internal/cfg/migrate.go`). It is driven by the same table: an entry whose `Replacement` is `(none)` is deleted, and one whose `Replacement` names a key (`[cell] winkit_ssh_port`) is renamed or moved there. Forms the table cannot express (a whole-table rename, a value rewrite such as `use_ollama = true` to `provider = "ollama"`) need a case in `MigrateTOML` plus a test in `migrate_test.go`; `TestMigrateTOML_KitchenSink_LoadsClean` must keep loading with zero deprecations.

To remove a key for good: delete the table entry, the old struct field and the merge code in the same change, and move consumers to the new field.

## Commands

Set cobra's `Deprecated` field on the old command, pointing at the new one:

```go
var oldCmd = &cobra.Command{
	Use:        "old",
	Deprecated: "use `cell new` instead",
	RunE:       newCmd.RunE,
}
```

Cobra hides it from help and prints `Command "old" is deprecated, <message>` to stderr on every run.

## Flags

Cobra-parsed flags: `cmd.Flags().MarkDeprecated("old-flag", "use --new-flag instead")`. Cobra hides the flag and prints `Flag --old-flag has been deprecated, <message>` when used.

Flags stripped from argv by `cellBoolFlags` in `cmd/root.go` (agent subcommands set `DisableFlagParsing`) never reach cobra, so `MarkDeprecated` does not fire for them; warn explicitly where the flag is scanned.
