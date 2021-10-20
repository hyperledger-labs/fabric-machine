#!/usr/bin/env bash
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script initializes the docker swarm, creates the overlay network and starts the dockers in
# the VM from where it is run.
set -e

# Arguments
DOCKER_COMPOSE_FILE=$1
DOCKERS=$2

printf "\n=== Setting up dockers in $HOSTNAME $HLNETWORK_IP_ADDRESS ===\n\n"

docker swarm init --advertise-addr $HLNETWORK_IP_ADDRESS
docker network create --driver overlay --attachable hlnetwork

docker-compose -f $DOCKER_COMPOSE_FILE up -d $DOCKERS
