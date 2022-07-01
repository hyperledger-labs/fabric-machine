#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

set -ex

GO_USER=$1

# Workaround for the scenario where the default libvirt network settings conflict with our
# host server settings.
sed -i '/^#/!s/^/#/' /etc/systemd/resolved.conf
systemctl restart systemd-resolved.service && sleep 10s
systemctl status systemd-resolved.service
systemd-resolve google.com
systemd-resolve us.archive.ubuntu.com
systemd-resolve download.docker.com
nslookup us.archive.ubuntu.com
nslookup download.docker.com

# Setup paths for Hyperledger Fabric.
goroot=/opt/go
gopath=/opt/gopath
mkdir -p $goroot $gopath
chown ${GO_USER}:${GO_USER} $goroot
chown ${GO_USER}:${GO_USER} $gopath
