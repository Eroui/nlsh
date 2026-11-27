#!/bin/zsh -f
# Drives a real interactive zsh through a pseudo-terminal, using the real nlsh
# binary against a fake model server. Run: zsh test/widget_test.zsh
emulate -L zsh
zmodload zsh/zpty
root=${0:A:h:h}
port=$(( 20000 + RANDOM % 20000 ))
python3 -I $root/test/fake_model.py $port & srv=$!
trap 'kill $srv 2>/dev/null; zpty -d sh 2>/dev/null' EXIT
sleep 0.5

export NLSH_PROVIDER=ollama NLSH_BASE_URL=http://127.0.0.1:$port/v1 NLSH_NO_KEYCHAIN=1 NLSH_CONFIG=/nonexistent/x.json
export NLSH_BIN=$root/bin/nlsh TERM=xterm-256color
fails=0

zpty sh zsh -f -i
zpty -w sh "PS1='READY> '; source $root/shell/nlsh.zsh; cd $root"

expect() {  # expect <label> <pattern>
  local buf='' chunk start=$SECONDS
  while (( SECONDS - start < 8 )); do
    if zpty -r -t sh chunk; then buf+=$chunk; else sleep 0.05; fi
    if [[ $buf == *$2* ]]; then print "ok   - $1"; return 0; fi
  done
  print "FAIL - $1"; print -r -- "  output was: ${(q+)buf[-400,-1]}"; (( fails++ )); return 1
}

expect "shell started" 'READY> '

# 1. "? request" + Enter shows a suggestion, Enter again runs it.
zpty -n -w sh $'? print a marker\r'
expect "suggestion shown" 'Enter: run'
zpty -n -w sh $'\r'
expect "Enter runs the suggested command" 'NLSH_42'

# 2. Esc restores the request instead of running anything.
zpty -n -w sh $'? print a marker\r'
expect "suggestion shown again" 'Enter: run'
zpty -n -w sh $'\e'; sleep 0.8
expect "Esc cancels" 'cancelled'
# Line is now "? print a marker": go to start, delete "?", prefix "echo RESTORED".
zpty -n -w sh $'\x01\x04echo RESTORED\r'
expect "original request was restored" $'RESTORED print a marker\r\n'

# 3. Dangerous command is flagged even when the model says "safe".
zpty -n -w sh $'? wipe the build folder\r'
expect "danger flagged" '[DANGER]'
zpty -n -w sh $'\e'; sleep 0.8
zpty -n -w sh $'\x15\r'   # Ctrl+U clears, Enter on empty line

# 4. No command possible: message shown, nothing replaced.
zpty -n -w sh $'? impossible: make coffee\r'
expect "explains when no command fits" "can't make coffee"

# 5. Normal commands are untouched.
zpty -n -w sh $'\x15echo plain_$((1+1))\r'
expect "normal Enter still works" 'plain_2'

(( fails == 0 )) && print "all widget tests passed" || { print "$fails failed"; exit 1 }
