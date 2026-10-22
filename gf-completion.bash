#!/usr/bin/env bash
# Don't spew "gf: command not found" if the binary isn't on $PATH
complete -W "\$(command -v gf >/dev/null 2>&1 && command gf -list 2>/dev/null)" gf
