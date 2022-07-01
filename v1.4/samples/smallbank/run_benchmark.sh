#!/usr/bin/env bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script contains the workflow for running a benchmark. Make sure to run setup_benchmark.sh
# before running this script.
set -e

scripts_dir=$(realpath ../../scripts/fabric)
caliper_cli_dir=$(realpath ../../caliper-cli)
benchmark_dir=$(realpath .)
benchmark=$(basename $benchmark_dir)

fabric_version=amd64-1.4.5-snapshot-11ff99170
fabric_ca_version=1.4.5
fabric_baseimage_version=0.4.18

echo "================================================="
echo "Generating cryptographic artifacts ..."
echo "================================================="
$scripts_dir/generate-artifacts.sh $benchmark

echo "================================================="
echo "Bringing up Fabric network ..."
echo "================================================="
$scripts_dir/fabric.py --fabric-version=$fabric_version --fabric-ca-version=$fabric_ca_version --fabric-baseimage-version=$fabric_baseimage_version -m restart -v vms.txt -n network.txt -d docker-compose.yaml

echo "================================================="
echo "Running $benchmark benchmark using Caliper CLI ..."
echo "================================================="
cd $caliper_cli_dir
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
echo "Collecting Fabric Machine peer log ..."
echo "================================================="
$scripts_dir/fm_simulator.py --blocks-file=peer1.org1.example.com.log --peer-log=peer2.org1.example.com.log

echo "================================================="
echo "Collecting peer statistics ..."
echo "================================================="
$scripts_dir/extract_stats.py --log-files='peer*.example.com.log'

echo "================================================="
echo "Processing peer statistics ..."
echo "================================================="
$scripts_dir/process_stats.py -m combinestats-perpeer

echo "================================================="
echo "All done!"
echo "================================================="
