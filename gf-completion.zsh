compdef _gf gf

function _gf {
    local gf_bin dir

    # Look the binary up when completing rather than calling `gf` directly:
    # aliases are expanded when this function is defined, so a `gf` alias
    # (e.g. oh-my-zsh's `gf='git fetch'`) would otherwise be run instead.
    if (( $+commands[gf] )); then
        gf_bin=$commands[gf]
    else
        # Fall back to where `go get` / `go install` put the binary
        for dir in ${(s.:.)${GOPATH:-$HOME/go}}; do
            if [[ -x $dir/bin/gf ]]; then
                gf_bin=$dir/bin/gf
                break
            fi
        done
    fi

    [[ -n $gf_bin ]] || return 1

    _arguments "1: :($("$gf_bin" -list 2>/dev/null))"
}
