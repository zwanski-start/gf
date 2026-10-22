#compdef gf

# zsh completion for gf
#
# Either source this file from your .zshrc (after your framework, e.g.
# oh-my-zsh or prezto, has been loaded), or copy it into a directory in
# your $fpath with the name _gf.

function _gf {
    local -a patterns
    # 'command' skips any 'gf' alias (e.g. the 'git fetch' alias that
    # oh-my-zsh and prezto define) and runs the gf binary itself
    patterns=(${(f)"$(command gf -list 2>/dev/null)"})

    _arguments \
        '-dump[print the grep command rather than executing it]' \
        '-list[list available patterns]' \
        '-save[save a pattern]' \
        "1:pattern:(${patterns[*]})" \
        '2:file:_files'
}

if [[ "${funcstack[1]}" == "_gf" ]]; then
    # Autoloaded from $fpath by compinit
    _gf "$@"
else
    # Sourced; make sure the completion system is loaded first
    if (( ! $+functions[compdef] )); then
        autoload -Uz compinit
        compinit
    fi
    compdef _gf gf

    # oh-my-zsh and prezto alias gf to 'git fetch'; zsh expands the alias
    # before completing, so gf's completion is never used while it exists
    if [[ "${aliases[gf]}" == git\ * ]]; then
        print -u2 "gf-completion: 'gf' is aliased to '${aliases[gf]}'; run 'unalias gf' after loading oh-my-zsh/prezto to use gf"
    fi
fi
