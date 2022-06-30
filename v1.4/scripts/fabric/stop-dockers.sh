#!/usr/bin/env bash
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script stops all the dockers in the VM from where it is run.
set -e

# Arguments
DOCKERS=$1

printf "\n=== Shutting down dockers in $HOSTNAME $HLNETWORK_IP_ADDRESS ===\n\n"
for d in $DOCKERS; do
    echo $d
done
echo ""

(docker stop $DOCKERS &> /dev/null || true) && (docker rm -v $DOCKERS &> /dev/null || true)
dev_dockers=$(docker ps -a -q --filter "name=dev")
(docker stop $dev_dockers &> /dev/null || true) && (docker rm -v $dev_dockers &> /dev/null || true)
docker image rm $(docker image ls -q dev-*) &> /dev/null || true
#docker volume prune --force || true
docker network rm hlnetwork &> /dev/null || true
docker swarm leave --force &> /dev/null || true
