#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

set -ex

GO_USER=$1

# Workaround for the scenario where the default libvirt network settings conflict with our
# host server settings.
sed -i '/^#/!s/^/#/' /etc/systemd/resolved.conf
systemctl restart systemd-resolved.service && sleep 5s
systemctl status systemd-resolved.service
#nslookup google.com

# Setup paths for Hyperledger Fabric.
goroot=/opt/go
gopath=/opt/gopath
mkdir -p $goroot $gopath
chown ${GO_USER}:${GO_USER} $goroot
chown ${GO_USER}:${GO_USER} $gopath
