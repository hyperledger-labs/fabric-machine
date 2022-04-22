#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

# Command-line parameters.
if [[ -n "$1" ]]; then
    DOCKER_USER=$1
else
    DOCKER_USER=$USER
fi

out=$(docker --version)
if [[ $? -eq 0 ]]; then
   echo "docker is already installed: $out"
fi
out=$(docker-compose --version)
if [[ $? -eq 0 ]]; then
   echo "docker-compose is already installed: $out"
fi

set -e

docker_version=5:19.03.15~3-0~ubuntu-bionic
docker_compose_version=1.27.0
while true; do
    echo "Do you want to continue with installation of docker $docker_version and docker-compose $docker_compose_version for user $DOCKER_USER"
    read -p "(sudo access needed; updates system files /lib/systemd/system/docker.service.d/docker.conf, /etc/default/docker) ? " ans
    case $ans in
        [Yy]* ) break;;
        [Nn]* ) exit;;
        * ) echo "Please answer yes or no.";;
    esac
done

echo "================================================="
echo "Installing Docker ..."
echo "================================================="
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
sudo apt-get update -qq
sudo apt-get install -y docker-ce=${docker_version}  # in case we need to set the version

# Configure docker
sudo tee /etc/default/docker << EOF
DOCKER_OPTS="-r=true --api-cors-header='*' -H tcp://0.0.0.0:2375 -H unix:///var/run/docker.sock"
EOF
sudo mkdir -p /lib/systemd/system/docker.service.d
sudo tee /lib/systemd/system/docker.service.d/docker.conf << EOF
[Service]
EnvironmentFile=-/etc/default/docker
ExecStart=
ExecStart=/usr/bin/dockerd \$DOCKER_OPTS
EOF

sudo systemctl daemon-reload
sudo service docker restart
sudo usermod -a -G docker $DOCKER_USER  # Add the installation user to the docker group

echo "================================================="
echo "Installing Docker Compose ..."
echo "================================================="
curl -L https://github.com/docker/compose/releases/download/${docker_compose_version}/docker-compose-`uname -s`-`uname -m` | sudo tee /usr/local/bin/docker-compose > /dev/null
sudo chmod +x /usr/local/bin/docker-compose

echo "================================================="
echo "docker $docker_version and docker-compose $docker_compose_version for user $DOCKER_USER installed successfully!"
echo "================================================="