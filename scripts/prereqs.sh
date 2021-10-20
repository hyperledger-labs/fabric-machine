#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This script installs all the prerequisites for this repo.
set -e

sudo ./install_golang.sh $USER
sudo ./install_docker.sh $USER

nvm_version=0.36.0
nodejs_version=8.13.0
echo "================================================="
echo "Installing NVM $nvm_version ..."
echo "================================================="
curl -o- https://raw.githubusercontent.com/creationix/nvm/v${nvm_version}/install.sh | bash +x
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm bash completion.

echo "================================================="
echo "Installing NodeJS $nodejs_version ..."
echo "================================================="
nvm install $nodejs_version
npm config set strict-ssl false
npm config set registry https://registry.npmjs.org/

echo "================================================="
echo "Installing Caliper CLI ..."
echo "================================================="
caliper_dir=$(realpath ../caliper-cli)
mkdir -p $caliper_dir && cd $caliper_dir
npm install --only=prod @hyperledger/caliper-cli@0.3.2
npx caliper bind --caliper-bind-sut fabric:1.4.5
