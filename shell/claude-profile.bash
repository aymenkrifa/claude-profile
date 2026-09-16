# claude-profile -- bash integration.
#
# Gives every Claude Code account on this machine its own pair of commands:
#
#   claude-<name>   the CLI bound to that account's CLAUDE_CONFIG_DIR
#   vs<name>        a VS Code window bound to the same account
#
# Accounts are discovered from ~/.claude-*/profile.env on every shell start,
# with shell builtins only, so adding one never means editing this file and
# nothing here spawns a process.
#
#   claude-profile shell-init bash      prints the source line for ~/.bashrc
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
#   CLAUDE_PROFILE_TERM_CLASS="ghostty:personal WezTerm:personal"
#
# The guard rail: with a terminal map set, a terminal belongs to exactly one
# class and only defines the commands for accounts of that class, so a personal
# terminal cannot reach a work account by accident. 'claude-profile run <name>'
# is the deliberate escape hatch.
#
# Written for bash 3.2, the version macOS still ships, so there are no
# associative arrays here: the maps are strings of pairs and the accounts are
# two parallel indexed arrays.

# --- helpers -------------------------------------------------------------

# __claude_pair() <pairs> <key> -- print the value for key, or fail.
__claude_pair() {
	local pair
	for pair in $1; do
		case "$pair" in
		"$2":?*) printf '%s' "${pair#*:}"; return 0 ;;
		esac
	done
	return 1
}

# __claude_first_value() <pairs> -- print the alphabetically first value, or
# nothing. Written without a case inside $( ), because bash 3.2 counts the ")"
# that closes a case pattern as closing the command substitution and rejects
# the whole file; and without sort, because that would put a process on the
# shell-start path for a one-line comparison.
__claude_first_value() {
	local pair value first=""
	for pair in $1; do
		value=${pair#*:}
		[ "$value" = "$pair" ] && continue
		[ -n "$value" ] || continue
		if [ -z "$first" ] || [ "$value" \< "$first" ]; then
			first=$value
		fi
	done
	[ -n "$first" ] || return 1
	printf '%s' "$first"
}

# --- discover the accounts ----------------------------------------------
__claude_names=()
__claude_classes=()
# Glob expansion is already sorted, so the commands come out in a stable order
# without sorting anything.
__claude_discover() {
	local d n c key val
	for d in "$HOME"/.claude-*/; do
		[ -f "$d/profile.env" ] || continue
		n=${d%/}
		n=${n##*/}
		n=${n#.claude-}
		c=$n
		while IFS='=' read -r key val; do
			case "$key" in
			class) [ -n "$val" ] && c=$val ;;
			esac
		done <"$d/profile.env"
		__claude_names+=("$n")
		__claude_classes+=("$c")
	done
}
__claude_discover

# __claude_class_of() <name> -- the class of one account.
__claude_class_of() {
	local i=0
	while [ $i -lt ${#__claude_names[@]} ]; do
		if [ "${__claude_names[$i]}" = "$1" ]; then
			printf '%s' "${__claude_classes[$i]}"
			return 0
		fi
		i=$((i + 1))
	done
	return 1
}

# Filtering is opt-in: a terminal map is what asks for it. Without one, an
# account added with a class nobody has mapped would otherwise get no commands
# at all, which looks exactly like the tool not working.
__claude_filtered=0
[ -n "${CLAUDE_PROFILE_TERM_CLASS:-}" ] && __claude_filtered=1

# Preserve the outer terminal identity so tmux shells keep the same mode.
if [ -z "${TMUX:-}" ]; then
	export OUTER_TERM_PROGRAM="${TERM_PROGRAM:-default-terminal}"
fi
__claude_term="${OUTER_TERM_PROGRAM:-${TERM_PROGRAM:-default-terminal}}"

# --- decide which account this shell starts in ---------------------------
__CLAUDE_MODE=""
__CLAUDE_CLASS=""

if [ -n "${CLAUDE_CONFIG_DIR:-}" ]; then
	# Respect a profile already chosen by the launcher (e.g. the per-account
	# Claude Desktop wrappers resolve env via a login shell -- don't clobber it).
	__CLAUDE_MODE="${CLAUDE_CONFIG_DIR##*/}"
	__CLAUDE_MODE="${__CLAUDE_MODE#.claude-}"
elif [ "$__claude_filtered" = 1 ]; then
	__CLAUDE_CLASS=$(__claude_pair "$CLAUDE_PROFILE_TERM_CLASS" "$__claude_term") || __CLAUDE_CLASS=""
	if [ -z "$__CLAUDE_CLASS" ] && [ -n "${TERMINAL_EMULATOR:-}" ]; then
		__CLAUDE_CLASS=$(__claude_pair "$CLAUDE_PROFILE_TERM_CLASS" "$TERMINAL_EMULATOR") || __CLAUDE_CLASS=""
	fi
	if [ -z "$__CLAUDE_CLASS" ]; then
		__CLAUDE_CLASS="${CLAUDE_PROFILE_FALLBACK_CLASS:-}"
	fi
	if [ -z "$__CLAUDE_CLASS" ]; then
		__CLAUDE_CLASS=$(__claude_first_value "$CLAUDE_PROFILE_TERM_CLASS") || __CLAUDE_CLASS=""
	fi
	__CLAUDE_MODE=$(__claude_pair "${CLAUDE_PROFILE_CLASS_DEFAULT:-}" "$__CLAUDE_CLASS") || __CLAUDE_MODE="$__CLAUDE_CLASS"
	# A renamed-away default must not leave the shell pointing at nothing: fall
	# back to any account of this class.
	if ! __claude_class_of "$__CLAUDE_MODE" >/dev/null; then
		__CLAUDE_MODE=""
		for __claude_i in "${__claude_names[@]}"; do
			if [ "$(__claude_class_of "$__claude_i")" = "$__CLAUDE_CLASS" ]; then
				__CLAUDE_MODE="$__claude_i"
				break
			fi
		done
		unset __claude_i
	fi
	[ -n "$__CLAUDE_MODE" ] && export CLAUDE_CONFIG_DIR="$HOME/.claude-$__CLAUDE_MODE"
else
	# Unfiltered. Point the shell at an account only when that is unambiguous:
	# an explicit default, or the single account on the machine. With several
	# and no default, CLAUDE_CONFIG_DIR stays unset and the claude stub asks.
	if [ -n "${CLAUDE_PROFILE_DEFAULT:-}" ] && __claude_class_of "$CLAUDE_PROFILE_DEFAULT" >/dev/null; then
		__CLAUDE_MODE="$CLAUDE_PROFILE_DEFAULT"
	elif [ ${#__claude_names[@]} -eq 1 ]; then
		__CLAUDE_MODE="${__claude_names[0]}"
	fi
	[ -n "$__CLAUDE_MODE" ] && export CLAUDE_CONFIG_DIR="$HOME/.claude-$__CLAUDE_MODE"
fi

if [ -z "$__CLAUDE_CLASS" ] && [ -n "$__CLAUDE_MODE" ]; then
	__CLAUDE_CLASS=$(__claude_class_of "$__CLAUDE_MODE") || __CLAUDE_CLASS=""
fi

# --- a claude-<name> / vs<name> pair per visible account -----------------
__CLAUDE_PROFILE_CMDS=""
for __claude_n in "${__claude_names[@]}"; do
	if [ "$__claude_filtered" = 1 ]; then
		[ "$(__claude_class_of "$__claude_n")" = "$__CLAUDE_CLASS" ] || continue
	fi
	eval "claude-$__claude_n() { CLAUDE_CONFIG_DIR=\"\$HOME/.claude-$__claude_n\" command claude \"\$@\"; }"
	eval "vs$__claude_n() { CLAUDE_CONFIG_DIR=\"\$HOME/.claude-$__claude_n\" code --user-data-dir \"\$HOME/.vscode-$__claude_n\" \"\$@\"; }"
	__CLAUDE_PROFILE_CMDS="${__CLAUDE_PROFILE_CMDS:+$__CLAUDE_PROFILE_CMDS }claude-$__claude_n"
done
unset __claude_n

# Shadow bare 'claude' so that an account has to be named. Under 'auto' this
# happens only when the choice is real: with one account in view there is
# nothing to disambiguate and shadowing it would just be in the way. Set
# always to keep the prompt even then, never to leave 'claude' alone.
__claude_count=$(set -- $__CLAUDE_PROFILE_CMDS; echo $#)
__claude_stub=0
case "${CLAUDE_PROFILE_STUB_CLAUDE:-auto}" in
always) __claude_stub=1 ;;
never) __claude_stub=0 ;;
*) [ "$__claude_count" -gt 1 ] && __claude_stub=1 ;;
esac
[ "$__claude_count" -eq 0 ] && __claude_stub=0

if [ "$__claude_stub" = 1 ]; then
	claude() {
		local list
		list=${__CLAUDE_PROFILE_CMDS// / | }
		printf "Pick an account instead of bare 'claude': %s\n" "$list" >&2
		if [ -n "$__CLAUDE_CLASS" ] && [ "$__claude_filtered" = 1 ]; then
			printf "(this is a '%s' terminal -- 'claude-profile ls' shows every account)\n" "$__CLAUDE_CLASS" >&2
		else
			printf "('claude-profile ls' shows every account)\n" >&2
		fi
		return 1
	}
fi

if type complete >/dev/null 2>&1; then
	complete -W "ls add set rename repath rm run desktop path doctor shell-init ${__claude_names[*]}" claude-profile
fi

# Only needed while sourcing. __claude_filtered and __CLAUDE_PROFILE_CMDS are
# not in this list: the claude stub reads both at the time it runs.
unset __claude_term __claude_stub __claude_count __claude_names __claude_classes
unset -f __claude_discover __claude_pair __claude_first_value __claude_class_of 2>/dev/null

# Tells claude-profile that these commands exist, so it can point at
# claude-<name> instead of at the install instructions.
export CLAUDE_PROFILE_SHELL=1
