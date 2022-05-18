#!/usr/bin/env bash
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script gathers logs of all the dockers in the VM from where it is run.
set -e

# Arguments
LOG_DIR=$1
DOCKERS=$2

for docker in $DOCKERS; do
    printf "\n=== Getting docker $docker log from $HOSTNAME $HLNETWORK_IP_ADDRESS ===\n\n"
    docker logs $docker 1>$LOG_DIR/$docker.log 2>&1
done
