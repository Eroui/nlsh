# nlsh zsh integration: type "? <what you want>" and press Enter.
# The line is replaced by a suggested command. Enter runs it, Esc cancels
# (puts your request back so you can rephrase). Nothing runs on its own.
#
# Enable by adding this to ~/.zshrc:
#   source /path/to/terminal/shell/nlsh.zsh

[[ -o interactive ]] || return 0

# Find the binary: $NLSH_BIN, then ../bin/nlsh next to this file, then PATH.
if [[ -z $NLSH_BIN ]]; then
  typeset -g NLSH_BIN=${${(%):-%x}:A:h:h}/bin/nlsh
  [[ -x $NLSH_BIN ]] || NLSH_BIN=nlsh
fi

typeset -g _nlsh_original=''
typeset -gi _nlsh_active=0

_nlsh_reset() {
  _nlsh_active=0
  _nlsh_original=''
  region_highlight=("${(@)region_highlight:#*nlsh*}")
}

_nlsh_accept_line() {
  if (( ! _nlsh_active )) && [[ $BUFFER == '?'* ]]; then
    local query=${BUFFER#\?}
    query=${query#"${query%%[![:space:]]*}"}   # trim leading spaces
    if [[ -z ${query//[[:space:]]/} ]]; then
      zle -M 'nlsh: type a request after "?", e.g.  ? show the 5 biggest files here'
      return
    fi

    zle -M "nlsh: thinking about \"$query\"..."
    zle -R

    local out rc
    out=$(NLSH_SHELL=zsh command $NLSH_BIN suggest --format=zsh -- "$query" 2>&1)
    rc=$?
    if (( rc != 0 )); then
      zle -M "${out:-nlsh: failed (exit $rc)}"
      return
    fi

    local -a parts
    parts=("${(@0)out}")
    local cmd=$parts[1] explain=$parts[2] risk=$parts[3]

    if [[ -z $cmd ]]; then
      zle -M "nlsh: ${explain:-no command suggested}"
      return
    fi

    _nlsh_original=$BUFFER
    _nlsh_active=1
    BUFFER=$cmd
    CURSOR=$#BUFFER

    local tag color
    case $risk in
      danger)  tag='DANGER'; color='fg=red,bold' ;;
      caution) tag='caution'; color='fg=yellow' ;;
      *)       tag='safe'; color='' ;;
    esac
    region_highlight=("${(@)region_highlight:#*nlsh*}")
    [[ -n $color ]] && region_highlight+=("0 $#BUFFER $color memo=nlsh")

    zle -M "[$tag] $explain"$'\n''Enter: run   Esc: cancel   (or edit it first)'
    return
  fi

  _nlsh_reset
  zle .accept-line
}

_nlsh_escape() {
  if (( _nlsh_active )); then
    BUFFER=$_nlsh_original
    CURSOR=$#BUFFER
    _nlsh_reset
    zle -M 'nlsh: cancelled'
  elif [[ $KEYMAP == viins ]]; then
    zle vi-cmd-mode
  fi
}

_nlsh_line_init() { _nlsh_reset }

zle -N accept-line _nlsh_accept_line
zle -N _nlsh_escape
autoload -Uz add-zle-hook-widget
add-zle-hook-widget line-init _nlsh_line_init

# Bare Esc. zsh waits $KEYTIMEOUT (default 0.4s) to tell it apart from
# Esc-prefixed keys like arrows or Alt+b, so those keep working.
bindkey -M emacs '^[' _nlsh_escape
bindkey -M viins '^[' _nlsh_escape
