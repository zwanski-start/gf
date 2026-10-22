compdef _gf gf

function _gf {
    # Don't spew "gf: command not found" if the binary isn't on $PATH
    (( $+commands[gf] )) || return 1
    _arguments "1: :($(command gf -list 2>/dev/null))"
}
