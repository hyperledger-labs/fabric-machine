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
chaincode_dir=src/chaincode/$benchmark/go/v2/shim

echo "================================================="
echo "Pulling prebuilt docker images ..."
echo "================================================="
docker pull hyperledger/fabric-ccenv:2.2.3 && docker tag hyperledger/fabric-ccenv:2.2.3 hyperledger/fabric-ccenv:latest

echo "================================================="
echo "Getting $benchmark benchmark files ..."
echo "================================================="
cd /tmp && rm -rf $caliper_benchmarks_dir && git clone https://github.com/hyperledger/caliper-benchmarks.git
cd $caliper_benchmarks_dir
mkdir -p $benchmark_dir/$chaincode_dir/

git checkout v0.3.2
cp benchmarks/scenario/$benchmark/smallbankOperations.js $benchmark_dir/workload.js

git checkout 7f05117
cp src/fabric/scenario/$benchmark/go/* $benchmark_dir/$chaincode_dir/

cd $benchmark_dir/$chaincode_dir/
GO111MODULE=on go mod vendor

echo "================================================="
echo "Applying local patches to benchmark files ..."
echo "================================================="


echo "================================================="
echo "All done!"
echo "================================================="
