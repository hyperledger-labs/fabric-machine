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

fabric_version=amd64-2.2.3-snapshot-56b368905
fabric_ca_version=1.5.0
fabric_baseimage_version=0.4.22

IFS='-' read -r arch fabric_major_version tmp <<< "$fabric_version"
if [[ -z "$fabric_major_version" ]]; then
    fabric_major_version=$fabric_version
fi

echo "================================================="
echo "Generating cryptographic artifacts ..."
echo "================================================="
FABRIC_VERSION=$fabric_major_version FABRIC_CA_VERSION=$fabric_ca_version FABRIC_BASEIMAGE_VERSION=$fabric_baseimage_version $scripts_dir/generate-artifacts.sh $benchmark
sleep 10s

echo "================================================="
echo "Bringing up Fabric network ..."
echo "================================================="
$scripts_dir/fabric.py --fabric-version=$fabric_version --fabric-ca-version=$fabric_ca_version --fabric-baseimage-version=$fabric_baseimage_version -m restart -v vms.txt -n network.txt -d docker-compose.yaml

echo "================================================="
echo "Creating and joining channel on Fabric network ..."
echo "================================================="
$scripts_dir/create-channel.sh mychannel
$scripts_dir/join-channel.sh mychannel "Org1MSP:peer0.org1.example.com Org1MSP:peer1.org1.example.com Org2MSP:peer0.org2.example.com Org2MSP:peer1.org2.example.com"

echo "================================================="
echo "Deploying chaincode on Fabric network ..."
echo "================================================="
$scripts_dir/chaincode.sh -i -c mychannel "Org1MSP:peer0.org1.example.com Org2MSP:peer0.org2.example.com" \
   smallbank 1.0 /opt/gopath/src/github.com/smallbank/go/v2/shim "AND('Org1MSP.member', 'Org2MSP.member')"

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
