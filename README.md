# claude-profile

One command for the Claude Code accounts on this machine.

Each account lives in its own config directory, `~/.claude-<name>`, holding its
own credentials, settings, history and projects. `claude-profile` creates,
renames and removes those accounts along with everything that hangs off them —
the shell commands, the Claude Desktop instance, the VS Code window, the MCP
server paths and the Desktop session records.

```
$ claude-profile ls
PROFILE   CLASS     ACCOUNT                            NOTES
personal  personal  you@example.com                    active here, desktop, personal account
acme      work      (not signed in)                    desktop, work account (Acme)
work      work      dev@acme.com  [Acme]               desktop, work account (Acme)
```

## Install

```sh
make install          # builds and installs to ~/.local/bin/claude-profile
```

Go 1.26+, no third-party dependencies.

## Commands

| | |
|---|---|
| `ls [-q]` | accounts, their class, and who each is signed in as |
| `add <name> [--class work\|personal] [--label ...] [--from <profile>] [--no-desktop] [--login]` | create, or adopt an existing directory |
| `set <name> [--class ...] [--label ...] [--desktop=false]` | change metadata; regenerates the Desktop entry |
| `rename <old> <new> [--dry-run] [--rewrite-history] [--force]` | rename everywhere |
| `repath <name> --from <old> [--dry-run] [--force]` | finish a rename: rewrite an old name still written inside history |
| `rm <name> [--purge] [-y]` | unregister; `--purge` deletes the data |
| `run <name> [-- args...]` | run the CLI against one account |
| `desktop <name> [args...]` | launch that account's Claude Desktop instance |
| `path <name>` | print its config directory |
| `doctor [name] [--deep]` | report path references that point at nothing |

Flags may be typed before or after the positional arguments.

## What a profile owns

| path | what |
|---|---|
| `~/.claude-<name>` | `CLAUDE_CONFIG_DIR` — credentials, settings, history, projects |
| `~/.config/Claude-<name>` | Claude Desktop's Electron user-data-dir |
| `~/.vscode-<name>` | VS Code user-data-dir, opened by the `vs<name>` function |
| `~/.local/bin/claude-desktop-<name>` | launcher shim |
| `~/.local/share/applications/claude-desktop-<name>.desktop` | app-menu entry |

A directory counts as a profile only if it contains a `profile.env` marker:

```
class=work
label=work account (Acme)
```

The marker is what stops unrelated dotfile directories from being listed as
accounts, and it is cheap enough that `~/.zshrc` reads it with shell builtins on
every shell start.

## class, and why it exists

`class` decides which terminals may reach an account. `~/.zshrc` maps a terminal
to one class and defines `claude-<name>` and `vs<name>` only for profiles of
that class, so a personal terminal cannot reach a work account by accident.
`claude-profile run <name>` is the deliberate escape hatch.

## rename

Renaming is the reason this is a program rather than a shell script. The old
name is not only a directory: it is baked into files inside three different
directory trees. `rename` sorts every reference it finds and only rewrites what
has to change:

- **config** — the live settings the CLI reads: MCP server commands
  (`.claude.json`), the `statusLine` hook (`settings.json`), plugin install
  locations (`plugins/*.json`). Rewritten.
- **session** — Claude Desktop's Code-tab records, whose `cwd` / `claudeDir`
  point into the Electron user-data-dir. Rewritten, or the app lists sessions it
  can no longer open.
- **history** — transcripts, `history.jsonl`, `file-history`, debug logs. These
  are a record of what happened; rewriting them would falsify it, and nothing
  reads a path out of them. Left alone unless you pass `--rewrite-history`.
- **volatile** — shell snapshots, daemon locks, temp files. Regenerated on the
  next run.

Anything else near the top of the profile that still names the old path is
reported rather than silently missed.

```sh
claude-profile rename work g4 --dry-run   # every move and edit, changes nothing
claude-profile rename work g4
```

Binary files are never rewritten, whatever the flags. The replacement path is
usually a different length, so substituting inside one shifts every following
byte and corrupts anything with an internal offset or length prefix — the `.pyc`
caches a job leaves behind, for instance. They are reported instead.

Only `claude-code-sessions/*/*/*.json` is touched inside the Electron
user-data-dir. The rest of it is LevelDB and IndexedDB, where the same
length-changing edit would break the store.

### repath

`rename` leaves history alone by default, so afterwards the transcripts still
name the old profile. `repath` is the second half, for when you decide you want
them changed after all:

```sh
claude-profile repath g4 --from work --dry-run
claude-profile repath g4 --from work
```

It refuses if `~/.claude-<old>` still exists — that situation is a rename, not a
repath. Only absolute paths are rewritten: a transcript quoting
`~/.claude-work` from an old shell config is describing a line that once
existed, and changing it would make the record wrong.

Safety: it refuses if the new name is taken anywhere, and refuses while any
process still has the profile open (checked through `/proc`, override with
`--force`). The plan is computed before anything is touched, moves happen before
rewrites, and every file is written atomically with its original mode — these
sit next to credentials at `0600`.

## doctor

Reports absolute references to a profile directory that no longer exists —
usually the residue of a rename done by hand. Only live config counts as a
fault; `--deep` also searches history and caches, and reports what it finds
there separately, because a transcript naming a path that used to exist is a
record rather than a fault.

## Layout

```
cmd/claude-profile/     CLI: one file per command
internal/profile/       discovery and the profile.env marker
internal/paths/         where every artefact of a profile lives
internal/rewrite/       the rename engine: classify, plan, apply
internal/desktop/       launcher shim and .desktop entry generation
internal/inuse/         which processes hold a profile open
internal/fsx/           atomic writes, moves
```

```sh
make        # fmt, vet, test, build
make test
```
