# claude-profile -- fish integration.
#
# Gives every Claude Code account on this machine its own pair of commands:
#
#   claude-<name>   the CLI bound to that account's CLAUDE_CONFIG_DIR
#   vs<name>        a VS Code window bound to the same account
#
# Accounts are discovered from ~/.claude-*/profile.env on every shell start,
# with builtins only, so adding one never means editing this file and nothing
# here spawns a process.
#
#   claude-profile shell-init fish      prints the source line for config.fish
#
# Optional configuration, set BEFORE sourcing this file:
#
#   CLAUDE_PROFILE_DEFAULT         the account a shell starts in.
#   CLAUDE_PROFILE_TERM_CLASS      terminal -> class, as name:value pairs.
#                                  Setting it turns on the guard rail below.
#                                  Unset (the default) means every account is
#                                  available in every terminal.
#   CLAUDE_PROFILE_CLASS_DEFAULT   class -> the account a shell of that class
#                                  starts in, same spelling.
#   CLAUDE_PROFILE_FALLBACK_CLASS  the class for a terminal the map does not
#                                  name. Defaults to the first class it names.
#   CLAUDE_PROFILE_STUB_CLAUDE     auto (default) | always | never -- whether
#                                  bare 'claude' is replaced by a stub that
#                                  makes you name an account. auto does it only
#                                  when more than one account is in view.
#
#   set -g CLAUDE_PROFILE_TERM_CLASS ghostty:personal WezTerm:personal
#
# A list and a single space-separated string both work: quoting a fish list
# joins it with spaces, which is what the reader below splits on.
#
# The guard rail: with a terminal map set, a terminal belongs to exactly one
# class and only defines the commands for accounts of that class, so a personal
# terminal cannot reach a work account by accident. 'claude-profile run <name>'
# is the deliberate escape hatch.

# --- helpers -------------------------------------------------------------

# __claude_pair <pairs> <key> -- print the value for key, or fail.
function __claude_pair -a pairs key
    for p in (string split " " -- "$pairs")
        set -l kv (string split -m 1 ":" -- $p)
        if test (count $kv) -eq 2; and test "$kv[1]" = "$key"; and test -n "$kv[2]"
            echo $kv[2]
            return 0
        end
    end
    return 1
end

# __claude_first_value <pairs> -- print the alphabetically first value, or
# nothing.
#
# Alphabetical, not as-written: zsh accepts the map as an associative array,
# which has no inherent order, so this is the only rule the three integrations
# can agree on. fish has no string comparison in test, so this is the one place
# a process is spawned -- and only when a terminal map names no fallback class
# and the terminal is not in the map.
function __claude_first_value -a pairs
    set -l values
    for p in (string split " " -- "$pairs")
        set -l kv (string split -m 1 ":" -- $p)
        if test (count $kv) -eq 2; and test -n "$kv[2]"
            set -a values $kv[2]
        end
    end
    if test (count $values) -eq 0
        return 1
    end
    set -l sorted (printf '%s\n' $values | sort)
    echo $sorted[1]
end

# --- discover the accounts ----------------------------------------------
# Kept as name:class pairs so the lookup above serves for these too. Glob
# expansion is already sorted, so the commands come out in a stable order
# without sorting anything.
set -g __claude_profiles
for d in $HOME/.claude-*/
    if not test -f $d/profile.env
        continue
    end
    set -l n (string replace -r '^.*/\.claude-' '' -- (string replace -r '/$' '' -- $d))
    set -l c $n
    while read -l line
        set -l kv (string split -m 1 "=" -- (string trim -- $line))
        if test (count $kv) -eq 2; and test "$kv[1]" = class; and test -n "$kv[2]"
            set c (string trim -- $kv[2])
        end
    end <$d/profile.env
    set -a __claude_profiles "$n:$c"
end

# Filtering is opt-in: a terminal map is what asks for it. Without one, an
# account added with a class nobody has mapped would otherwise get no commands
# at all, which looks exactly like the tool not working.
set -g __claude_filtered 0
if set -q CLAUDE_PROFILE_TERM_CLASS; and test -n "$CLAUDE_PROFILE_TERM_CLASS"
    set __claude_filtered 1
end

# Preserve the outer terminal identity so tmux shells keep the same mode.
if not set -q TMUX
    if set -q TERM_PROGRAM
        set -gx OUTER_TERM_PROGRAM $TERM_PROGRAM
    else
        set -gx OUTER_TERM_PROGRAM default-terminal
    end
end
set -l __claude_term $OUTER_TERM_PROGRAM
if test -z "$__claude_term"
    set __claude_term default-terminal
end

# --- decide which account this shell starts in ---------------------------
set -g __CLAUDE_MODE ""
set -g __CLAUDE_CLASS ""

if set -q CLAUDE_CONFIG_DIR; and test -n "$CLAUDE_CONFIG_DIR"
    # Respect a profile already chosen by the launcher (e.g. the per-account
    # Claude Desktop wrappers resolve env via a login shell -- don't clobber it).
    set __CLAUDE_MODE (string replace -r '^\.claude-' '' -- (string replace -r '^.*/' '' -- (string replace -r '/$' '' -- $CLAUDE_CONFIG_DIR)))
else if test $__claude_filtered -eq 1
    set __CLAUDE_CLASS (__claude_pair "$CLAUDE_PROFILE_TERM_CLASS" $__claude_term; true)
    if test -z "$__CLAUDE_CLASS"; and set -q TERMINAL_EMULATOR
        set __CLAUDE_CLASS (__claude_pair "$CLAUDE_PROFILE_TERM_CLASS" $TERMINAL_EMULATOR; true)
    end
    if test -z "$__CLAUDE_CLASS"; and set -q CLAUDE_PROFILE_FALLBACK_CLASS
        set __CLAUDE_CLASS $CLAUDE_PROFILE_FALLBACK_CLASS
    end
    if test -z "$__CLAUDE_CLASS"
        set __CLAUDE_CLASS (__claude_first_value "$CLAUDE_PROFILE_TERM_CLASS"; true)
    end
    set __CLAUDE_MODE (__claude_pair "$CLAUDE_PROFILE_CLASS_DEFAULT" $__CLAUDE_CLASS; true)
    if test -z "$__CLAUDE_MODE"
        set __CLAUDE_MODE $__CLAUDE_CLASS
    end
    # A renamed-away default must not leave the shell pointing at nothing: fall
    # back to any account of this class.
    if test -z (__claude_pair "$__claude_profiles" $__CLAUDE_MODE; true)
        set __CLAUDE_MODE ""
        for p in $__claude_profiles
            set -l kv (string split -m 1 ":" -- $p)
            if test "$kv[2]" = "$__CLAUDE_CLASS"
                set __CLAUDE_MODE $kv[1]
                break
            end
        end
    end
    if test -n "$__CLAUDE_MODE"
        set -gx CLAUDE_CONFIG_DIR $HOME/.claude-$__CLAUDE_MODE
    end
else
    # Unfiltered. Point the shell at an account only when that is unambiguous:
    # an explicit default, or the single account on the machine. With several
    # and no default, CLAUDE_CONFIG_DIR stays unset and the claude stub asks.
    if set -q CLAUDE_PROFILE_DEFAULT; and test -n (__claude_pair "$__claude_profiles" "$CLAUDE_PROFILE_DEFAULT"; true)
        set __CLAUDE_MODE $CLAUDE_PROFILE_DEFAULT
    else if test (count $__claude_profiles) -eq 1
        set __CLAUDE_MODE (string split -m 1 ":" -- $__claude_profiles[1])[1]
    end
    if test -n "$__CLAUDE_MODE"
        set -gx CLAUDE_CONFIG_DIR $HOME/.claude-$__CLAUDE_MODE
    end
end

if test -z "$__CLAUDE_CLASS"; and test -n "$__CLAUDE_MODE"
    set __CLAUDE_CLASS (__claude_pair "$__claude_profiles" $__CLAUDE_MODE; true)
end

# --- a claude-<name> / vs<name> pair per visible account -----------------
# env, not a bare call: it runs the real binary rather than re-entering the
# claude stub defined below.
set -g __CLAUDE_PROFILE_CMDS
for p in $__claude_profiles
    set -l kv (string split -m 1 ":" -- $p)
    set -l n $kv[1]
    if test $__claude_filtered -eq 1; and test "$kv[2]" != "$__CLAUDE_CLASS"
        continue
    end
    eval "function claude-$n --description 'Claude Code on the $n account'
        env CLAUDE_CONFIG_DIR=\$HOME/.claude-$n claude \$argv
    end"
    eval "function vs$n --description 'VS Code on the $n account'
        env CLAUDE_CONFIG_DIR=\$HOME/.claude-$n code --user-data-dir \$HOME/.vscode-$n \$argv
    end"
    set -a __CLAUDE_PROFILE_CMDS claude-$n
end

# Shadow bare 'claude' so that an account has to be named. Under 'auto' this
# happens only when the choice is real: with one account in view there is
# nothing to disambiguate and shadowing it would just be in the way. Set
# always to keep the prompt even then, never to leave 'claude' alone.
set -l __claude_stub 0
set -l __claude_count (count $__CLAUDE_PROFILE_CMDS)
switch "$CLAUDE_PROFILE_STUB_CLAUDE"
    case always
        set __claude_stub 1
    case never
        set __claude_stub 0
    case '*'
        if test $__claude_count -gt 1
            set __claude_stub 1
        end
end
if test $__claude_count -eq 0
    set __claude_stub 0
end

if test $__claude_stub -eq 1
    function claude --description "refuses: name an account instead"
        set -l list (string join " | " $__CLAUDE_PROFILE_CMDS)
        echo "Pick an account instead of bare 'claude': $list" >&2
        if test -n "$__CLAUDE_CLASS"; and test $__claude_filtered -eq 1
            echo "(this is a '$__CLAUDE_CLASS' terminal -- 'claude-profile ls' shows every account)" >&2
        else
            echo "('claude-profile ls' shows every account)" >&2
        end
        return 1
    end
end

set -l __claude_names
for p in $__claude_profiles
    set -a __claude_names (string split -m 1 ":" -- $p)[1]
end
complete -c claude-profile -f -a "ls add set rename repath rm run desktop path doctor shell-init $__claude_names"

# Only needed while sourcing. __claude_filtered and __CLAUDE_PROFILE_CMDS are
# not erased: the claude stub reads both at the time it runs.
functions -e __claude_pair __claude_first_value
set -e __claude_profiles

# Tells claude-profile that these commands exist, so it can point at
# claude-<name> instead of at the install instructions.
set -gx CLAUDE_PROFILE_SHELL 1
