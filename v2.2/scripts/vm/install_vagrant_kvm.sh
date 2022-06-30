#!/bin/bash
#
# Copyright Xilinx Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

out=$(vagrant --version)
if [[ $? -eq 0 ]]; then
    echo "vagrant is already installed: $out"
fi
out=$(virsh --version)
if [[ $? -eq 0 ]]; then
    echo "libvirt/kvm is already installed: $out"
fi


while true; do
    read -p "Do you want to continue with installation of vagrant and libvirt/kvm (sudo access needed)? " ans
    case $ans in
        [Yy]* ) break;;
        [Nn]* ) exit;;
        * ) echo "Please answer yes or no.";;
    esac
done

sudo apt-get install -y vagrant

sudo apt-get build-dep vagrant ruby-libvirt
sudo apt-get install -y qemu libvirt-bin ebtables dnsmasq-base
sudo apt-get install -y libxslt-dev libxml2-dev libvirt-dev zlib1g-dev ruby-dev


LIBVIRT_GROUP=$(grep "unix_sock_group" /etc/libvirt/libvirtd.conf | awk '{print $3}')
LIBVIRT_GROUP="${LIBVIRT_GROUP%\"}"
LIBVIRT_GROUP="${LIBVIRT_GROUP#\"}"
while true; do
    read -p "Going to add user $USER to group $LIBVIRT_GROUP which is required to create/run VMs. Do you want to continue (sudo access needed)? " ans
    case $ans in
        [Yy]* ) sudo usermod -aG $LIBVIRT_GROUP $USER ; break;;
        [Nn]* ) exit;;
        * ) echo "Please answer yes or no.";;
    esac
done


vagrant plugin install vagrant-libvirt
vagrant plugin install vagrant-bindfs
