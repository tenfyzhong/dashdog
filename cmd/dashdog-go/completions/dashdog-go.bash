#!/usr/bin/env bash

_dashdog_go_complete() {
    local cur opts
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    allopts="-p --pkg -c --config --log --path --cfbundle --path-regex --bundle-pattern --bundle-replace -h --help"
    
    if [[ "$cur" = "-"* ]]; then
        opts="$allopts"
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi

    case "$prev" in
        --log)
            opts="debug info warn error off"
            COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
            ;;
        --path)
            COMPREPLY=( $(compgen -d) )
            ;;
    esac
    return 0
}

complete -F _dashdog_go_complete dashdog
