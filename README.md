# claude-profile

One command for the Claude Code accounts on this machine.

Each account lives in its own config directory, `~/.claude-<name>`, holding its
own credentials, settings, history and projects. `claude-profile` creates,
renames and removes those accounts along with everything that hangs off them —
the shell commands, the Claude Desktop instance, the VS Code window, the MCP
server paths and the Desktop session records.

Linux and macOS. Unofficial, and not affiliated with Anthropic.

```
$ claude-profile ls
PROFILE   CLASS     ACCOUNT               BROWSER        NOTES
acme      work      (not signed in)       (default)      desktop, work account (Acme)
personal  personal  you@example.com       open-chromium  active here, desktop, personal account
work      work      dev@acme.com  [Acme]  (default)      desktop, work account (Acme)
```

BROWSER is the `env.BROWSER` of that account's `settings.json`, the command
Claude Code spawns to open a URL; `(default)` means the account names none and
URLs follow the system opener. Not every link obeys it: the Claude in Chrome
flow resolves a browser from its own list (Chrome, Brave, Arc, Edge, Chromium,
Vivaldi, Opera, in that order), so an account can name one browser here and
still hand that flow another.

## Install

```sh
curl -fsSL https://claudeprofile.aymenkrifa.com/install.sh | sh
```

Then reload your shell (`exec zsh`, or open a new terminal), and that's it.

That URL serves this repository's `install.sh` from `main`, unchanged (see
`web/`); the same script is at
`https://raw.githubusercontent.com/aymenkrifa/claude-profile/main/install.sh`.

Installs the binary and the shell integrations under `~/.local`, never with
`sudo`, after verifying the archive against the release's `checksums.txt`. It
then hooks the integration into the startup file of the shell `$SHELL` names —
`~/.zshrc`, `~/.bashrc` (`~/.bash_profile` on macOS) or fish's `config.fish` —
as one marked block that puts `~/.local/bin` on PATH and sources the
integration. That block is the only thing written outside `~/.local`; rerunning
the installer replaces it, deleting it undoes it, and a startup file that
already sources the integration by hand is left alone. `--prefix DIR`,
`--version vX.Y.Z`, `--no-shell` and `--no-modify-rc` all work: pass them after
`sh -s --`.

To update later, `claude-profile update` (or `--check` to only look). It reruns
the installer against the prefix the binary lives in, refreshes the
startup-file block if the installer wrote one, and never adds one that isn't
there. A build from source says so and points at `git pull && make install`
instead.

From a clone instead:

```sh
make install          # ~/.local/bin/claude-profile + the shell integrations
```

Go 1.26+ to build, no third-party dependencies. `claude-profile desktop`
additionally needs Claude Desktop installed — a `claude-desktop` command on
PATH on Linux, `Claude.app` in `/Applications` or `~/Applications` on macOS.

## Shell integration

The integration is what gives each account its own commands. The install
script wires it up; after `make install`, or with `--no-modify-rc`, print the
line to add yourself:

```sh
claude-profile shell-init             # for whichever shell $SHELL names
claude-profile shell-init bash        # or name one: zsh, bash, fish
```

```
claude-<name>   the CLI bound to that account's CLAUDE_CONFIG_DIR
vs<name>        a VS Code window bound to the same account
```

Accounts are discovered from `~/.claude-*/profile.env` on every shell start,
with shell builtins only, so adding one never means editing the file and
nothing on that path spawns a process. That is also why `shell-init` prints a
`source` line rather than something to `eval`.

zsh, bash and fish, one file each, installed together whatever shell you use.
The bash one is written for bash 3.2 — the version macOS still ships — so it
keeps the accounts in parallel indexed arrays rather than using associative
arrays. All three read the same configuration and behave the same way; a test
compares them case by case.

By default every account is available in every terminal. Set a terminal map
and the `class` guard rail below turns on.

### class, and why it exists

`class` decides which terminals may reach an account. Map a terminal to a class
and the shell defines `claude-<name>` and `vs<name>` only for accounts of that
class, so a personal terminal cannot reach a work account by accident.
`claude-profile run <name>` is the deliberate escape hatch.

Set these **before** the `source` line. The maps are `name:value` pairs:

```zsh
# zsh, bash
CLAUDE_PROFILE_TERM_CLASS="ghostty:personal WezTerm:personal"
CLAUDE_PROFILE_CLASS_DEFAULT="work:acme personal:personal"
CLAUDE_PROFILE_FALLBACK_CLASS=work
```

```fish
# fish
set -g CLAUDE_PROFILE_TERM_CLASS ghostty:personal WezTerm:personal
set -g CLAUDE_PROFILE_CLASS_DEFAULT work:acme personal:personal
set -g CLAUDE_PROFILE_FALLBACK_CLASS work
```

zsh also accepts the associative-array spelling
(`typeset -gA CLAUDE_PROFILE_TERM_CLASS=( ghostty personal )`), and fish
accepts either a list or one space-separated string.

| | |
|---|---|
| `CLAUDE_PROFILE_TERM_CLASS` | terminal → class. Setting it is what turns filtering on |
| `CLAUDE_PROFILE_CLASS_DEFAULT` | class → the account a shell of that class starts in |
| `CLAUDE_PROFILE_FALLBACK_CLASS` | the class for a terminal the map does not name |
| `CLAUDE_PROFILE_DEFAULT` | with no terminal map, the account a shell starts in |
| `CLAUDE_PROFILE_STUB_CLAUDE` | `auto` (default), `always` or `never` — whether bare `claude` refuses and makes you name an account. `auto` does so only when more than one is in view |

Filtering is opt-in because the alternative fails quietly: `add` defaults an
account's class to its own name, and an account whose class nobody has mapped
would get no commands at all — indistinguishable from the tool not working.
With no map, `CLAUDE_CONFIG_DIR` is set only when the choice is unambiguous (a
single account, or `CLAUDE_PROFILE_DEFAULT`); otherwise bare `claude` lists the
accounts and asks.

## Commands

| | |
|---|---|
| `ls [-q]` | accounts, their class, who each is signed in as, and which browser each opens URLs with |
| `add <name> [--class work\|personal] [--label ...] [--from <profile>] [--no-desktop] [--login]` | create, or adopt an existing directory |
| `set <name> [--class ...] [--label ...] [--desktop=false]` | change metadata; regenerates the Desktop entry |
| `rename <old> <new> [--dry-run] [--rewrite-history] [--force]` | rename everywhere |
| `repath <name> --from <old> [--dry-run] [--force]` | finish a rename: rewrite an old name still written inside history |
| `rm <name> [--purge] [-y]` | unregister; `--purge` deletes the data |
| `run <name> [-- args...]` | run the CLI against one account |
| `desktop <name> [args...]` | launch that account's Claude Desktop instance |
| `path <name>` | print its config directory |
| `doctor [name] [--deep]` | report path references that point at nothing |
| `shell-init [zsh\|bash\|fish]` | print the line to add to that shell's startup file |
| `update [--check] [--version vX.Y.Z] [--force]` | install the latest release over this one, through the same installer |

Flags may be typed before or after the positional arguments.

## What a profile owns

| path | what |
|---|---|
| `~/.claude-<name>` | `CLAUDE_CONFIG_DIR` — credentials, settings, history, projects |
| `~/.config/Claude-<name>` (Linux)<br>`~/Library/Application Support/Claude-<name>` (macOS) | Claude Desktop's Electron user-data-dir |
| `~/.vscode-<name>` | VS Code user-data-dir, opened by the `vs<name>` function |
| `~/.local/bin/claude-desktop-<name>` | launcher shim |
| `~/.local/share/applications/claude-desktop-<name>.desktop` (Linux)<br>`~/Applications/Claude (<name>).app` (macOS) | application-menu entry |

The two that differ are the two the platforms disagree about: Electron puts an
app's data where the platform says it goes, and an application-menu entry is a
`.desktop` file on one and an `.app` bundle on the other. The macOS bundle is a
wrapper whose executable re-enters `claude-profile desktop <name>`, so the
launch flags live in one place, the same as the Linux shim.

A directory counts as a profile only if it contains a `profile.env` marker:

```
class=work
label=work account (Acme)
```

The marker is what stops unrelated dotfile directories from being listed as
accounts, and it is cheap enough for the shell integration to read with
builtins on every shell start.

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

A path is only replaced where it appears as a whole path. Renaming `a` to `ab`
makes the old directory a prefix of every profile whose name starts with `a`, so
a blind substitution would turn a reference to `~/.claude-abc` — a profile that
is not being renamed — into `~/.claude-abbc`. A match counts only when the
characters on either side of it cannot be continuing a longer name.

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
process still has the profile open. The plan is computed before anything is
touched, moves happen before rewrites, and every file is written atomically
with its original mode — these sit next to credentials at `0600`.

That in-use check reads `/proc` on Linux and shells out to `lsof` on macOS, and
it fails closed: if it cannot look — no `/proc`, no `lsof`, an unsupported
platform — the command refuses rather than proceeding, because "nothing holds
this profile" and "I could not tell" must not reach the decision looking the
same. `--force` overrides either.

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
internal/paths/         where every artefact of a profile lives, per platform
internal/rewrite/       the rename engine: classify, plan, apply
internal/desktop/       launcher shim, .desktop entry and .app bundle
internal/inuse/         which processes hold a profile open (/proc, lsof)
internal/fsx/           atomic writes, moves
shell/                  the zsh, bash and fish integrations
install.sh              downloads a release, verifies it, installs it
test/e2e.sh             install, then add / rename / rm a profile in real shells (make e2e)
```

```sh
make        # fmt, vet, test, build
make test
make dist   # the release archives for all four platforms
```

`paths.Layout` carries the platform as a field rather than reading
`runtime.GOOS` at each use, so the macOS layout — and the `.app` bundle
generated from it — is exercised by tests running on Linux, and the other way
round. What that cannot cover is the code that actually calls `lsof` and
launches `Claude.app`; CI runs the suite on macOS for those.

`rm --purge` asks for confirmation before it removes anything at all, including
the `profile.env` marker: an aborted purge has to leave the account exactly as
it was, not unregistered.

## License

MIT.
