#!/usr/bin/env bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script cleans up the benchmark directory by deleting files generated during a previous run.
# You must run setup_benchmark.sh after this.
set -e

CLEAN_LOGS=false

POSITIONAL=()
while [[ $# -gt 0 ]]; do
key="$1"
case $key in
    # Clean logs as well.
    -l)
    CLEAN_LOGS=true
    shift # past argument
    ;;
    *)    # unknown option
    POSITIONAL+=("$1") # save it in an array for later
    shift # past argument
    ;;
esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

caliper_benchmarks_dir=/tmp/caliper-benchmarks
benchmark_dir=$(realpath .)
benchmark=$(basename $benchmark_dir)
chaincode_dir=src/chaincode/$benchmark/go/v2/shim

# Fabric binaries and cryptographic artifacts
rm -rf bin config crypto-config
rm -f genesis.block mychannel.tx

# Docker images
docker image rm hyperledger/fabric-ccenv:2.2.3 hyperledger/fabric-ccenv:latest &> /dev/null || true

# Benchmark
rm -rf $chaincode_dir
rm -f workload.js

# Clean logs
if [ ${CLEAN_LOGS} = true ]; then
    rm -f *.log report.html committer_stats_*.txt combined_stats.txt
fi

echo "================================================="
echo "All done!"
echo "================================================="
