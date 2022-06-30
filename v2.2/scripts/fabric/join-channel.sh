#!/usr/bin/env bash
# This script joins peers to an already created channel of a Fabric network.
# Usage (peer0.org1 joins mychannel):
#   ./join-channel.sh mychannel "Org1MSP:peer0.org1.example.com"
set -e

# Arguments
CHANNEL=$1
TARGET_PEERS=$2  # Format is "msp:peer" e.g. Org1MSP:peer0.org1.example.com

for tp in $TARGET_PEERS; do
    IFS=':' read -r msp peer <<< "$tp"
    IFS='.' read -r tmp org <<< "$peer"

    printf "\n=== Joining channel $CHANNEL : $peer ===\n\n"

    docker exec -e CORE_PEER_ID=$peer -e CORE_PEER_ADDRESS=$peer:7051 -e CORE_PEER_LOCALMSPID=$msp -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/crypto-config/peerOrganizations/$org/users/Admin@$org/msp cli.example.com peer channel join -b /etc/hyperledger/$CHANNEL.block 1>joinch.log 2>&1
done