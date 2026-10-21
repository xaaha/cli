# Changelog

All notable changes to this module are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the module uses
[semantic versioning](https://semver.org).

## Unreleased

### Changed

- Help for a command without subcommands shows its `Long` or `Short`
  description and a USAGE line, and no longer ends with a LEARN MORE hint
  about subcommands it does not have.

### Added

- Runnable examples on pkg.go.dev, a fuller package doc and README.

## v0.2.0 - 2026-10-21

### Added

- `Command.FlagsFirst` stops reading flags at the command's first positional
  argument, so free text that looks like flags reaches `Run` as typed.
- zsh and bash completion stop offering flags after the first positional of a
  `FlagsFirst` command, including one nested under another `FlagsFirst`
  command.

### Fixed

- Bash completion reads `--flag==value`, which bash splits into `--flag`,
  `==` and `value`, as a value for `--flag`.
- After `--flag=` followed by a space, bash completion completes a new word
  instead of the flag's value.
- A quoted value containing `=` is no longer taken as a `--flag=value`
  separator by bash completion.
- The bash completion helper keeps its variables local and matches typed
  words literally.

## v0.1.0 - 2026-10-21

First release, extracted from [hulak](https://github.com/xaaha/hulak).

### Added

- `Command` trees with nested subcommands, aliases, hidden commands,
  positional argument descriptions and help examples.
- Flags in any position, before or after positionals and before a
  subcommand's name. `--` ends flag parsing and stops subcommand dispatch,
  and a lone `-` is a positional.
- Help at every level in a style like the gh CLI, with short and long flag
  pairs on one line, type hints that match the flag's type, and alphabetical
  command lists.
- `--help` and its short alias are added to every `FlagSet` that lacks them
  when the tree is linked. A command's own `--help` or `-h` stays the app's,
  `FlagAliases["help"]` changes or disables the short form, and
  `--help=false` runs the command.
- Help and flag errors name the root program and the full command path.
- `Config` on the root command for the whole tree: `FlagAliases`,
  `HiddenFlags`, `FlagCompleters`, `Completers`, `Out`, `Err` and `Color`.
  Maps you supply add to the defaults.
- Colour in help headers only when `Out` is a terminal and `NO_COLOR` is
  unset or empty, with `ColorAlways` and `ColorNever` to override.
- `WriteZshCompletion` and `WriteBashCompletion` generate scripts from the
  same tree, with `KindFile`, `KindDir` and app-registered `Completer` kinds
  for argument and flag values. Both halves of a flag pair share a completer.
- Bash completion understands `--flag=value`, `--` and a lone `-`, and
  completes only the command's positional after `--`.
- The completion writers return an error for unsafe names, unknown or
  incomplete completers, zsh function name collisions, an invalid help alias
  and the first failed write.
