#!/usr/bin/env bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script contains the workflow for running a benchmark. Make sure to run setup_benchmark.sh
# before running this script.
set -e

scripts_dir=$(realpath ../../scripts)
caliper_dir=$(realpath ../../caliper-cli)
benchmark_dir=$(realpath .)
benchmark=$(basename $benchmark_dir)

echo "================================================="
echo "Generating cryptographic artifacts ..."
echo "================================================="
$scripts_dir/generate-artifacts.sh $benchmark

echo "================================================="
echo "Bringing up Fabric network ..."
echo "================================================="
$scripts_dir/fabric.py -m restart -v vms.txt -n network.txt -d docker-compose.yaml

echo "================================================="
echo "Running $benchmark benchmark using Caliper CLI ..."
echo "================================================="
cd $caliper_dir
npx caliper launch master --caliper-workspace=$benchmark_dir \
    --caliper-projectconfig=caliper-config.yaml --caliper-benchconfig=caliper-config.yaml \
    --caliper-networkconfig=caliper-fabric-config.yaml

echo "================================================="
echo "Collecting logs from dockers ..."
echo "================================================="
cd $benchmark_dir
$scripts_dir/fabric.py -m log -v vms.txt -n network.txt -d docker-compose.yaml

echo "================================================="
echo "Bringing down Fabric network ..."
echo "================================================="
$scripts_dir/fabric.py -m stop -v vms.txt -n network.txt -d docker-compose.yaml

echo "================================================="
echo "All done!"
echo "================================================="
