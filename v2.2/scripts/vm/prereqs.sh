#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script installs all the prerequisites for this repo.
set -e

VM_USER=$USER

# Install some basic utilities.
echo "================================================="
echo "Installing basic utilities (curl, g++, make, etc.) ..."
echo "================================================="
sudo apt-get update -qq
sudo apt-get install -y \
    apt-transport-https \
    build-essential \
    ca-certificates \
    curl \
    g++ \
    git \
    gnupg \
    jq \
    libtool \
    make \
    nfs-common \
    python \
    python-pip \
    sshfs \
    unzip
pip install pyyaml

./install_golang.sh $VM_USER
./install_docker.sh $VM_USER

nvm_version=0.36.0
nodejs_version=8.13.0
echo "================================================="
echo "Installing NVM $nvm_version ..."
echo "================================================="
curl -o- https://raw.githubusercontent.com/creationix/nvm/v${nvm_version}/install.sh | bash +x
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm bash completion.
echo "================================================="
echo "Installed NVM $nvm_version successfully!"
echo "================================================="

echo "================================================="
echo "Installing NodeJS $nodejs_version ..."
echo "================================================="
nvm install $nodejs_version
npm config set strict-ssl false
npm config set registry https://registry.npmjs.org/
echo "================================================="
echo "Installed NodeJS $nodejs_version successfully!"
echo "================================================="

caliper_version=0.3.2
echo "================================================="
echo "Installing Caliper CLI ${caliper_version} ..."
echo "================================================="
caliper_cli_dir=$(realpath ../../caliper-cli)
mkdir -p $caliper_cli_dir && cd $caliper_cli_dir
npm init -y
npm install --only=prod @hyperledger/caliper-cli@${caliper_version}
npx caliper bind --caliper-bind-sut fabric:1.4.5  # Fabric SDK 1.4.5 is compatible with both Fabric v1 and v2.
echo "================================================="
echo "Installed Caliper CLI ${caliper_version} successfully!"
echo "================================================="
