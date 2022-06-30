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

printf "\n=== Getting docker logs from $HOSTNAME $HLNETWORK_IP_ADDRESS ===\n\n"
for d in $DOCKERS; do
    echo $d
    docker logs $d 1>$LOG_DIR/$d.log 2>&1
done
echo ""