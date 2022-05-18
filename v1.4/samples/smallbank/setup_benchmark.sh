#!/usr/bin/env bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script sets up the files needed to run a benchmark. This should be run only once before
# run_benchmark.sh or when benchmark needs to be updated.
set -e

caliper_benchmarks_dir=/tmp/caliper-benchmarks
benchmark_dir=$(realpath .)
benchmark=$(basename $benchmark_dir)
chaincode_dir=src/chaincode/go

echo "================================================="
echo "Pulling prebuilt docker images ..."
echo "================================================="
docker pull hyperledger/fabric-ccenv:1.4.5 && docker tag hyperledger/fabric-ccenv:1.4.5 hyperledger/fabric-ccenv:latest

echo "================================================="
echo "Getting $benchmark benchmark files ..."
echo "================================================="
cd /tmp && rm -rf $caliper_benchmarks_dir && git clone https://github.com/hyperledger/caliper-benchmarks.git
cd $caliper_benchmarks_dir
git checkout v0.3.2

cd $benchmark_dir && mkdir -p $chaincode_dir/
cp $caliper_benchmarks_dir/benchmarks/scenario/$benchmark/smallbankOperations.js workload.js
cp $caliper_benchmarks_dir/src/fabric/scenario/$benchmark/go/$benchmark.go $chaincode_dir/

echo "================================================="
echo "Applying local patches to benchmark files ..."
echo "================================================="


echo "================================================="
echo "All done!"
echo "================================================="
