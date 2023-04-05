#!/bin/bash

ACTIONS=("deploy-vagrant")

function action_deploy-vagrant {    
    scp sms-bulk-mock-go vagrant@192.168.1.134:/var/www/bulk-mock/
}


function run_action {
    local ACTION_NAME="action_$1"
    if typeset -f "${ACTION_NAME}" > /dev/null; then
        $ACTION_NAME $2 $3
    else
        printf "\nAction [$1] is not available.\nAvailable actions: "
        echo "${ACTIONS[@]}"
        printf "\n\n"
        exit 1
    fi    
}

run_action $1 $2 $3