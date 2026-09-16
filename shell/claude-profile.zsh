# claude-profile -- zsh integration.
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
#   claude-profile shell-init      prints the source line for ~/.zshrc
#
# Optional configuration, set BEFORE sourcing this file:
#
#   CLAUDE_PROFILE_DEFAULT         the account a shell starts in.
#   CLAUDE_PROFILE_TERM_CLASS      terminal -> class. Setting it turns on the
#                                  guard rail described below. Unset (the
#                                  default) means every account is available
#                                  in every terminal.
#   CLAUDE_PROFILE_CLASS_DEFAULT   class -> the account a shell of that class
#                                  starts in.
#
# The two maps take either form:
#
#   CLAUDE_PROFILE_TERM_CLASS="ghostty:personal WezTerm:personal"
#   typeset -gA CLAUDE_PROFILE_TERM_CLASS=( ghostty personal WezTerm personal )
#
# The string of pairs is the portable one -- the bash and fish integrations
# read the same spelling -- and the associative array is the zsh-native one.
#   CLAUDE_PROFILE_FALLBACK_CLASS  the class for a terminal the map does not
#                                  name. Defaults to the first class it names.
#   CLAUDE_PROFILE_STUB_CLAUDE     auto (default) | always | never -- whether
#                                  bare 'claude' is replaced by a stub that
#                                  makes you name an account. auto does it only
#                                  when more than one account is in view.
#
# The guard rail: with a terminal map set, a terminal belongs to exactly one
# class and only defines the commands for accounts of that class, so a personal
# terminal cannot reach a work account by accident. 'claude-profile run <name>'
# is the deliberate escape hatch.

# --- discover the accounts ----------------------------------------------
typeset -gA __CLAUDE_PROFILE_CLASS=()
() {
  local d n key val
  for d in $HOME/.claude-*(N/); do
    [[ -f $d/profile.env ]] || continue
    n=${${d:t}#.claude-}
    __CLAUDE_PROFILE_CLASS[$n]=$n
    while IFS='=' read -r key val; do
      if [[ $key == class && -n $val ]]; then
        __CLAUDE_PROFILE_CLASS[$n]=$val
      fi
    done < $d/profile.env
  done
}

# --- normalise the configuration ----------------------------------------
# Each map arrives either as an associative array or as a string of name:value
# pairs, and the rest of the file reads only the internal copies.
typeset -gA __claude_term_class=() __claude_class_default=()
() {
  local name pair k v
  for name in TERM_CLASS CLASS_DEFAULT; do
    local -a pairs=()
    case "${(Pt)${:-CLAUDE_PROFILE_$name}}" in
    association*)
      for k in ${(kP)${:-CLAUDE_PROFILE_$name}}; do
        pairs+=("$k:${${(P)${:-CLAUDE_PROFILE_$name}}[$k]}")
      done
      ;;
    *)
      pairs=( ${=${(P)${:-CLAUDE_PROFILE_$name}}} )
      ;;
    esac
    for pair in $pairs; do
      k=${pair%%:*}; v=${pair#*:}
      [[ -n $k && -n $v && $k != $pair ]] || continue
      if [[ $name == TERM_CLASS ]]; then
        __claude_term_class[$k]=$v
      else
        __claude_class_default[$k]=$v
      fi
    done
  done
}

# Filtering is opt-in: a terminal map is what asks for it. Without one, an
# account added with a class nobody has mapped would otherwise get no commands
# at all, which looks exactly like the tool not working.
typeset -g __claude_filtered=0
(( ${#__claude_term_class} > 0 )) && __claude_filtered=1

# Preserve the outer terminal identity so tmux shells keep the same mode.
if [[ -z "${TMUX:-}" ]]; then
  export OUTER_TERM_PROGRAM="${TERM_PROGRAM:-default-terminal}"
fi
typeset -g __claude_term="${OUTER_TERM_PROGRAM:-${TERM_PROGRAM:-default-terminal}}"

# --- decide which account this shell starts in ---------------------------
typeset -g __CLAUDE_MODE=""
typeset -g __CLAUDE_CLASS=""

if [[ -n "${CLAUDE_CONFIG_DIR:-}" ]]; then
  # Respect a profile already chosen by the launcher (e.g. the per-account
  # Claude Desktop wrappers resolve env via a login shell -- don't clobber it).
  __CLAUDE_MODE="${${CLAUDE_CONFIG_DIR:A:t}#.claude-}"
elif (( __claude_filtered )); then
  __CLAUDE_CLASS="${__claude_term_class[$__claude_term]}"
  [[ -n $__CLAUDE_CLASS ]] || __CLAUDE_CLASS="${__claude_term_class[${TERMINAL_EMULATOR:-__none__}]}"
  if [[ -z $__CLAUDE_CLASS ]]; then
    __CLAUDE_CLASS="${CLAUDE_PROFILE_FALLBACK_CLASS:-}"
  fi
  if [[ -z $__CLAUDE_CLASS ]]; then
    # No explicit fallback: the alphabetically first class the map names, so an
    # unmapped terminal lands somewhere predictable rather than on whichever
    # key the hash happens to yield first. The array has to be a real array to
    # be subscripted -- ${${(ov)map}[1]} would slice a character off a string.
    () {
      local -a classes=( ${(ov)__claude_term_class} )
      __CLAUDE_CLASS="${classes[1]}"
    }
  fi
  __CLAUDE_MODE="${__claude_class_default[$__CLAUDE_CLASS]:-$__CLAUDE_CLASS}"
  # A renamed-away default must not leave the shell pointing at nothing: fall
  # back to any account of this class. Cleared before the search, so that a
  # class with no accounts at all ends up exporting nothing rather than a
  # CLAUDE_CONFIG_DIR naming a directory that does not exist.
  if [[ -z ${__CLAUDE_PROFILE_CLASS[$__CLAUDE_MODE]} ]]; then
    __CLAUDE_MODE=""
    () {
      local n
      for n in ${(ok)__CLAUDE_PROFILE_CLASS}; do
        if [[ ${__CLAUDE_PROFILE_CLASS[$n]} == $__CLAUDE_CLASS ]]; then
          __CLAUDE_MODE=$n
          break
        fi
      done
    }
  fi
  [[ -n $__CLAUDE_MODE ]] && export CLAUDE_CONFIG_DIR="$HOME/.claude-$__CLAUDE_MODE"
else
  # Unfiltered. Point the shell at an account only when that is unambiguous:
  # an explicit default, or the single account on the machine. With several and
  # no default, CLAUDE_CONFIG_DIR stays unset and the claude stub below asks.
  if [[ -n "${CLAUDE_PROFILE_DEFAULT:-}" && -n ${__CLAUDE_PROFILE_CLASS[$CLAUDE_PROFILE_DEFAULT]} ]]; then
    __CLAUDE_MODE="$CLAUDE_PROFILE_DEFAULT"
  elif (( ${#__CLAUDE_PROFILE_CLASS} == 1 )); then
    # ${(k)assoc[1]} would look up the key "1", and ${${(ok)assoc}[1]} would
    # slice the first character off the one key; assign to an array first.
    () {
      local -a names=( ${(ok)__CLAUDE_PROFILE_CLASS} )
      __CLAUDE_MODE="${names[1]}"
    }
  fi
  [[ -n $__CLAUDE_MODE ]] && export CLAUDE_CONFIG_DIR="$HOME/.claude-$__CLAUDE_MODE"
fi

[[ -n $__CLAUDE_CLASS ]] || __CLAUDE_CLASS="${__CLAUDE_PROFILE_CLASS[$__CLAUDE_MODE]:-}"

# --- a claude-<name> / vs<name> pair per visible account -----------------
typeset -ga __CLAUDE_PROFILE_CMDS=()
() {
  local n
  for n in ${(ok)__CLAUDE_PROFILE_CLASS}; do
    if (( __claude_filtered )); then
      [[ ${__CLAUDE_PROFILE_CLASS[$n]} == $__CLAUDE_CLASS ]] || continue
    fi
    functions[claude-$n]="CLAUDE_CONFIG_DIR=\"\$HOME/.claude-$n\" command claude \"\$@\""
    functions[vs$n]="CLAUDE_CONFIG_DIR=\"\$HOME/.claude-$n\" code --user-data-dir \"\$HOME/.vscode-$n\" \"\$@\""
    __CLAUDE_PROFILE_CMDS+=("claude-$n")
  done
}

# Shadow bare 'claude' so that an account has to be named. Under 'auto' this
# happens only when the choice is real: with one account in view there is
# nothing to disambiguate and shadowing it would just be in the way. Set
# always to keep the prompt even then, never to leave 'claude' alone.
typeset -g __claude_stub=0
case "${CLAUDE_PROFILE_STUB_CLAUDE:-auto}" in
  always) __claude_stub=1 ;;
  never)  __claude_stub=0 ;;
  *)      (( ${#__CLAUDE_PROFILE_CMDS} > 1 )) && __claude_stub=1 ;;
esac
# Never shadow it when there is nothing to offer instead.
(( ${#__CLAUDE_PROFILE_CMDS} == 0 )) && __claude_stub=0

if (( __claude_stub )); then
  function claude() {
    print -u2 "Pick an account instead of bare 'claude': ${(oj: | :)__CLAUDE_PROFILE_CMDS}"
    if [[ -n $__CLAUDE_CLASS ]] && (( __claude_filtered )); then
      print -u2 "(this is a '$__CLAUDE_CLASS' terminal -- 'claude-profile ls' shows every account)"
    else
      print -u2 "('claude-profile ls' shows every account)"
    fi
    return 1
  }
fi

compdef '_arguments "1:command:(ls add set rename repath rm run desktop path doctor shell-init)" "*:profile:(${(k)__CLAUDE_PROFILE_CLASS})"' claude-profile 2>/dev/null

# Only needed while sourcing. __claude_filtered and __CLAUDE_PROFILE_CMDS are
# not in this list: the claude stub reads both at the time it runs.
unset __claude_term __claude_stub __claude_term_class __claude_class_default

# Tells claude-profile that these commands exist, so it can point at
# claude-<name> instead of at the install instructions.
export CLAUDE_PROFILE_SHELL=1
