#!/usr/bin/env bash
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script generates cryptographic artifacts for use in a benchmark.

# Environment variables.
: "${FABRIC_VERSION:=1.4.5}"
: "${FABRIC_CA_VERSION:=1.4.5}"
: "${FABRIC_BASEIMAGE_VERSION:=0.4.18}"

# Command-line parameters.
BENCHMARK=$1

# Local variables.
[ -z "$BENCHMARK" ] && echo "Please specify benchmark as the first command-line parameter." && exit
scripts_dir=$(dirname $(realpath $0))
benchmark_dir=$scripts_dir/../samples/$BENCHMARK
cd $benchmark_dir

# Download Fabric binaries if not available in bin directory.
if [[ ! -d "bin" ]]; then
  curl -sSL http://bit.ly/2ysbOFE | bash -s -- ${FABRIC_VERSION} ${FABRIC_CA_VERSION} ${FABRIC_BASEIMAGE_VERSION} -ds
fi
rm -f hyperledger-fabric-*.tar.gz

# Remove old artifacts and generate new ones.
rm -rf crypto-config
rm -f genesis.block
rm -f mychannel.tx
./bin/cryptogen generate --config=crypto-config.yaml
./bin/configtxgen -configPath=./ -profile OrdererGenesis -outputBlock genesis.block -channelID syschannel
./bin/configtxgen -configPath=./ -profile ChannelConfig -outputCreateChannelTx mychannel.tx -channelID mychannel

# Rename the key files to deterministic names.
for KEY in $(find crypto-config -type f -name "*_sk"); do
  KEY_DIR=$(dirname ${KEY})
  mv ${KEY} ${KEY_DIR}/key.pem
done
