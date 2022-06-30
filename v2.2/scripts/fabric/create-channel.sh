#!/usr/bin/env bash
# This script creates a channel in an already setup Fabric network.
# Usage:
#   ./create-channel.sh mychannel
set -e

# Arguments
CHANNEL=$1

printf "\n=== Creating channel $CHANNEL ===\n\n"

docker exec cli.example.com peer channel create -o orderer0.example.com:7050 -c $CHANNEL -f /etc/hyperledger/$CHANNEL.tx --outputBlock /etc/hyperledger/$CHANNEL.block 1>createch.log 2>&1
sleep 2s
