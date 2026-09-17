# Bash completion for navune.
#
# Source this file from your shell, or install it as
# /usr/share/bash-completion/completions/navune (see `make install-completions`).

_navune_commands="analyze init version help"
_navune_global_flags="--help -h --version -V"
_navune_analyze_flags="--config --format --out --verbose -v -h --help"
_navune_formats="text json yaml mermaid"

_navune() {
    local cur prev cmd i
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # The subcommand is the first non-flag word after the program name.
    cmd=""
    for ((i = 1; i < COMP_CWORD; i++)); do
        case "${COMP_WORDS[i]}" in
            -*) ;;
            *) cmd="${COMP_WORDS[i]}"; break ;;
        esac
    done

    # Options that take a value: complete the value, not another flag.
    case "$prev" in
        --format)
            COMPREPLY=( $(compgen -W "$_navune_formats" -- "$cur") )
            return
            ;;
        --config|--out)
            COMPREPLY=( $(compgen -f -- "$cur") )
            return
            ;;
    esac

    # No subcommand yet: complete commands or global flags.
    if [[ -z "$cmd" ]]; then
        if [[ "$cur" == -* ]]; then
            COMPREPLY=( $(compgen -W "$_navune_global_flags" -- "$cur") )
        else
            COMPREPLY=( $(compgen -W "$_navune_commands" -- "$cur") )
        fi
        return
    fi

    case "$cmd" in
        analyze)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "$_navune_analyze_flags" -- "$cur") )
            else
                COMPREPLY=( $(compgen -d -- "$cur") )
            fi
            ;;
        init)
            COMPREPLY=( $(compgen -d -- "$cur") )
            ;;
        help)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "--help -h" -- "$cur") )
            else
                COMPREPLY=( $(compgen -W "$_navune_commands" -- "$cur") )
            fi
            ;;
        version)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "--help -h" -- "$cur") )
            fi
            ;;
    esac
}

complete -F _navune navune
