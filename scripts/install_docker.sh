#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
set -e

# Command-line parameters.
DOCKER_USER=$1

echo "================================================="
echo "Installing Docker ..."
echo "================================================="
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
apt-get update -qq
apt-get install -y docker-ce=5:19.03.15~3-0~ubuntu-bionic  # in case we need to set the version

# Configure docker
cat << EOF > /etc/default/docker
DOCKER_OPTS="-r=true --api-cors-header='*' -H tcp://0.0.0.0:2375 -H unix:///var/run/docker.sock"
EOF
mkdir -p /lib/systemd/system/docker.service.d
cat << EOF > /lib/systemd/system/docker.service.d/docker.conf
[Service]
EnvironmentFile=-/etc/default/docker
ExecStart=
ExecStart=/usr/bin/dockerd \$DOCKER_OPTS
EOF

systemctl daemon-reload
service docker restart
usermod -a -G docker $DOCKER_USER  # Add the installation user to the docker group

echo "================================================="
echo "Installing Docker Compose ..."
echo "================================================="
curl -L https://github.com/docker/compose/releases/download/1.27.0/docker-compose-`uname -s`-`uname -m` > /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
