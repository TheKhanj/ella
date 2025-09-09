_ella_module() {
	local cur prev cmds global_opts logs_opts run_opts start_opts
	COMPREPLY=()
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[COMP_CWORD - 1]}"

	cmds="run logs start stop restart reload list"
	global_opts="-h -v"

	logs_opts="-h -a -c"
	run_opts="-h -a -c -l"
	start_opts="-h -a -c"
	stop_opts="-h -a -c"
	restart_opts="-h -a -c"
	reload_opts="-h -a -c"
	list_opts="-h"

	if [[ $COMP_CWORD -eq 1 ]]; then
		COMPREPLY=($(compgen -W "${cmds} ${global_opts}" -- "$cur"))
		return 0
	fi

	local subcmd=""
	for word in "${COMP_WORDS[@]}"; do
		case "$word" in
		run | logs | start | stop | restart | reload | list)
			subcmd=$word
			break
			;;
		esac
	done

	if [[ $cur == -* ]]; then
		case "$subcmd" in
		run) COMPREPLY=($(compgen -W "${run_opts}" -- "$cur")) ;;
		logs) COMPREPLY=($(compgen -W "${logs_opts}" -- "$cur")) ;;
		start) COMPREPLY=($(compgen -W "${start_opts}" -- "$cur")) ;;
		stop) COMPREPLY=($(compgen -W "${stop_opts}" -- "$cur")) ;;
		restart) COMPREPLY=($(compgen -W "${restart_opts}" -- "$cur")) ;;
		reload) COMPREPLY=($(compgen -W "${reload_opts}" -- "$cur")) ;;
		list) COMPREPLY=($(compgen -W "${list_opts}" -- "$cur")) ;;
		*) COMPREPLY=($(compgen -W "${global_opts}" -- "$cur")) ;;
		esac
		return 0
	fi

	if [[ $prev == "-c" ]]; then
		COMPREPLY=($(compgen -f -- "$cur"))
		return 0
	fi

	if [[ -n "$subcmd" ]]; then
		local cfg=""
		for ((i = 1; i < COMP_CWORD; i++)); do
			if [[ "${COMP_WORDS[i]}" == "-c" && $((i + 1)) -lt $COMP_CWORD ]]; then
				cfg="${COMP_WORDS[i + 1]}"
				break
			fi
		done

		if [[ -n "$cfg" ]]; then
			COMPREPLY=($(compgen -W "$(ella list -c "$cfg")" -- "$cur"))
		else
			COMPREPLY=($(compgen -W "$(ella list)" -- "$cur"))
		fi
		return 0
	fi
}

complete -F _ella_module ella
