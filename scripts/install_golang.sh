#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
set -e

# Command-line parameters.
GO_USER=$1

goroot="/opt/go"
gopath="/opt/gopath"
go_version=1.12.12
echo "================================================="
echo "Installing Go ${go_version} ..."
echo "================================================="
go_url=https://storage.googleapis.com/golang/go${go_version}.linux-amd64.tar.gz
mkdir -p $goroot
curl -sL "$go_url" | (cd $goroot && tar --strip-components 1 -xz)

cat <<EOF >> /home/${GO_USER}/.bashrc

export GOROOT=$goroot
export GOPATH=$gopath
export PATH=\$PATH:$goroot/bin

EOF