#!/usr/bin/env bash
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script joins the docker swarm and the overlay network, and starts the dockers in the VM
# from where it is run.
set -e

# Arguments
DOCKER_SWARM_JOIN_CMD=$1
DOCKER_COMPOSE_FILE=$2
DOCKERS=$3

printf "\n=== Setting up dockers in $HOSTNAME $HLNETWORK_IP_ADDRESS ===\n\n"

$DOCKER_SWARM_JOIN_CMD
# Apply the following hack only when docker swarm mode is joined.
if [[ ! -z $DOCKER_SWARM_JOIN_CMD ]]; then
    # This is a hack to ensure hlnetwork is visible in the VM.
    docker run -itd --name test --net hlnetwork busybox
fi

docker-compose -f $DOCKER_COMPOSE_FILE up -d $DOCKERS
