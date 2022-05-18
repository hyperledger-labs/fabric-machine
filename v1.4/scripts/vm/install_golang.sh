#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

# Command-line parameters.
if [[ -n "$1" ]]; then
    GO_USER=$1
else
    GO_USER=$USER
fi
if [[ -n "$2" ]]; then
    GO_VER=$2
else
    GO_VER=1.14.12
fi
if [[ -n "$3" ]]; then
    GOROOT=$3
else
    GOROOT=/opt/go
fi
if [[ -n "$4" ]]; then
    GOPATH=$4
else
    GOPATH=/opt/gopath
fi

out=$(go version)
if [[ $? -eq 0 ]]; then
    go env
    echo "Go is already installed"
fi

set -e

while true; do
    echo "Do you want to continue with installation of Go ${GO_VER} for user ${GO_USER} with GOROOT=$GOROOT GOPATH=$GOPATH "
    read -p "(sudo access needed for go-dep installation and updates .bashrc with Go paths) ? " ans
    case $ans in
        [Yy]* ) break;;
        [Nn]* ) exit;;
        * ) echo "Please answer yes or no.";;
    esac
done

mkdir -p $GOROOT $GOPATH
chown -R ${GO_USER}:${GO_USER} $GOROOT $GOPATH

echo "================================================="
echo "Installing Go ${GO_VER} ..."
echo "================================================="
go_url=https://storage.googleapis.com/golang/go${GO_VER}.linux-amd64.tar.gz
curl -sL "$go_url" | (cd $GOROOT && tar --strip-components 1 -xz)
sudo apt-get install -y go-dep

cat <<EOF >> /home/${GO_USER}/.bashrc

export GOROOT=$GOROOT
export GOPATH=$GOPATH
export PATH=$GOROOT/bin:\$PATH

EOF

echo "================================================="
echo "Go $GO_VER for user $GO_USER with GOROOT=$GOROOT GOPATH=$GOPATH installed successfully!"
echo "================================================="
