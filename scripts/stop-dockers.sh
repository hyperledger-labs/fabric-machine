#!/usr/bin/env bash
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script stops all the dockers in the VM from where it is run.
set -e

# Arguments

printf "\n=== Shutting down dockers in $HOSTNAME $HLNETWORK_IP_ADDRESS ===\n\n"

(docker stop $(docker ps -a -q) || true) && (docker rm $(docker ps -a -q) || true)
docker image rm $(docker image ls -q dev-*) || true
docker volume prune --force || true
docker network rm hlnetwork || true
docker swarm leave --force || true