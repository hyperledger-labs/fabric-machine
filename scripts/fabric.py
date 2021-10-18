#!/usr/bin/env python
#
# Copyright Xilinx Inc. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# This program provides utilities to interact with a Hyperledger Fabric network (e.g., start/stop
# a network, gather logs, etc.).
#
# Assumptions:
#   1. VMs are already created and ssh'able.
#   2. This program itself is run from a VM.
import argparse
import glob
import os
import re
import subprocess
import sys

FABRIC_VERSION = ''
FABRIC_CA_VERSION = ''
FABRIC_BASEIMAGE_VERSION = ''

HOST_USERNAME = ''
HOST_HOSTNAME = ''
SSH_CMD = 'ssh -o StrictHostKeyChecking=no -i {key} -p {port} vagrant@{hostname}'
SCRIPT_DIR = '.'

# Returns the path to the key for ssh login.
# Arguments:
#   vm (string): name of the VM
#
# Returns:
#   a string containing the path to the ssh key
def get_ssh_key(vm):
    if re.match('hl-node.*', vm):
        return '/opt/gopath/src/github.com/hyperledger/fabric/vagrant/vbox/.vagrant-{host_hostname}/machines/{vm}/virtualbox/private_key'.format(
            host_hostname=HOST_HOSTNAME, vm=vm)
    else:
        return '/opt/gopath/src/github.com/hyperledger/fabric/vagrant/.ssh/id_rsa_{host_username}'.format(host_username=HOST_USERNAME)

# Runs a command and returns its output.
# Arguments:
#   cmd (string): contents of the command
#   num_tries (int): number of times to try in case of failures
#
# Returns:
#   a string containing the output of the command
def run_cmd(cmd, num_tries):
    print("=== Running cmd ===")
    ret = 0
    for i in range(num_tries):
        # Stream the output of the command to console while its running.
        output = ''
        process = subprocess.Popen([cmd], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, shell=True)
        while True:
            line = process.stdout.readline()
            if line:
                print(line.strip())
                output += line
                continue
            if process.poll() is not None:
                break  # break the loop only if line is empty and process has finished
        ret = process.returncode
        if ret == 0:
            break
        else:
            print("Cmd failed ...")

        # Wait for the command to finish, and then print its output to console.
        #try:
        #    output = subprocess.check_output([cmd], stderr=subprocess.STDOUT, shell=True)
        #    print(output)
        #except subprocess.CalledProcessError as error:
        #    print(error.output)
        #    sys.exit(error.returncode)

    if ret != 0:
        print("Cmd {0} unsuccessful, already tried {1} times and failed ...".format(cmd, i + 1))
        sys.exit(ret)
    return output

# Extracts the 'docker swarm join ...' command.
# Arguments:
#   text (string): text to search from
#
# Returns:
#   a string representing the join command
def extract_docker_swarm_join_cmd(text):
    token_re = re.compile(' docker swarm join ')
    lines = re.split('\n+', text)
    for i, line in enumerate(lines):
        match = token_re.search(line)
        if match:
            return line
            #return '\n'.join(lines[i:i+3])

# Starts the dockers after ssh'ing into the VM, considering it as the master VM.
# Arguments:
#   vm (string): name of the VM
#   hostname (string): IP address of the VM
#   port (string): ssh port
#   docker_compose_file (string): the docker compose file with docker configurations
#   dockers (string): space-separated list of dockers to start
#
# Returns:
#   a string representing the 'docker swarm join ...' command (for other VMs)
def start_master_vm(vm, hostname, port, docker_compose_file, dockers):
    env_vars = 'FABRIC_VERSION={0} FABRIC_CA_VERSION={1} FABRIC_BASEIMAGE_VERSION={2}'.format(
        FABRIC_VERSION, FABRIC_CA_VERSION, FABRIC_BASEIMAGE_VERSION)
    if vm == 'localhost':
        ssh_cmd = ''
        script_cmd = '{script_dir}/start-master-dockers.sh "{file}" "{dockers}"'.format(
            script_dir=SCRIPT_DIR, file=docker_compose_file, dockers=dockers)
    else:
        ssh_cmd = SSH_CMD.format(key=get_ssh_key(vm), hostname=hostname, port=port)
        script_cmd = '\'bash -ls\' < {script_dir}/start-master-dockers.sh \'"{file}" "{dockers}"\''.format(
            script_dir=SCRIPT_DIR, file=docker_compose_file, dockers=dockers)
    output = run_cmd(ssh_cmd + ' ' + env_vars + ' ' + script_cmd, 1)
    return extract_docker_swarm_join_cmd(output)

# Starts the dockers after ssh'ing into the VM, considering it as the slave VM.
# Arguments:
#   vm (string): name of the VM
#   hostname (string): IP address of the VM
#   port (string): ssh port
#   docker_swarm_join_cmd (string): command to join the docker swarm
#   docker_compose_file (string): the docker compose file with docker configurations
#   dockers (string): space-separated list of dockers to stop
#
# Returns:
#   a string containing the output of the ssh operation
def start_slave_vm(vm, hostname, port, docker_swarm_join_cmd, docker_compose_file, dockers):
    env_vars = 'FABRIC_VERSION={0} FABRIC_CA_VERSION={1} FABRIC_BASEIMAGE_VERSION={2}'.format(
        FABRIC_VERSION, FABRIC_CA_VERSION, FABRIC_BASEIMAGE_VERSION)
    if vm == 'localhost':
        ssh_cmd = ''
        script_cmd = '{script_dir}/start-slave-dockers.sh "{swarm_cmd}" "{file}" "{dockers}"'.format(
            script_dir=SCRIPT_DIR, swarm_cmd=docker_swarm_join_cmd, file=docker_compose_file, dockers=dockers)
    else:
        ssh_cmd = SSH_CMD.format(key=get_ssh_key(vm), hostname=hostname, port=port)
        script_cmd = '\'bash -ls\' < {script_dir}/start-slave-dockers.sh \'"{swarm_cmd}" "{file}" "{dockers}"\''.format(
            script_dir=SCRIPT_DIR, swarm_cmd=docker_swarm_join_cmd, file=docker_compose_file, dockers=dockers)
    return run_cmd(ssh_cmd + ' ' + env_vars + ' ' + script_cmd, 1)

# Stops all dockers after ssh'ing into the VM.
# Arguments:
#   vm (string): name of the VM
#   hostname (string): IP address of the VM
#   port (string): ssh port
#
# Returns:
#   a string containing the output of the ssh operation
def stop_vm(vm, hostname, port):
    if vm == 'localhost':
        ssh_cmd = ''
        script_cmd = '{script_dir}/stop-dockers.sh'.format(script_dir=SCRIPT_DIR)
    else:
        ssh_cmd = SSH_CMD.format(key=get_ssh_key(vm), hostname=hostname, port=port)
        script_cmd = '\'bash -ls\' < {script_dir}/stop-dockers.sh'.format(script_dir=SCRIPT_DIR)
    return run_cmd(ssh_cmd + ' ' + script_cmd, 1)

# Gets logs of all the dockers after ssh'ing into the VM.
# Arguments:
#   vm (string): name of the VM
#   hostname (string): IP address of the VM
#   port (string): ssh port
#   log_dir (string): the directory for log files
#   dockers (string): space-separated list of dockers
#
# Returns:
#   a string containing the output of the ssh operation
def get_docker_logs(vm, hostname, port, log_dir, dockers):
    if vm == 'localhost':
        ssh_cmd = ''
        script_cmd = '{script_dir}/get-docker-logs.sh "{log_dir}" "{dockers}"'.format(
            script_dir=SCRIPT_DIR, log_dir=log_dir, dockers=dockers)
    else:
        ssh_cmd = SSH_CMD.format(key=get_ssh_key(vm), hostname=hostname, port=port)
        script_cmd = '\'bash -ls\' < {script_dir}/get-docker-logs.sh "{log_dir}" "{dockers}"'.format(
            script_dir=SCRIPT_DIR, log_dir=log_dir, dockers=dockers)
    return run_cmd(ssh_cmd + ' ' + script_cmd, 1)

# Reads available VMs from a file.
# Arguments:
#   vms_file (string): Path glob pattern for files with VM configurations
#
# Returns:
#   a dictionary of VMs with their configuration details
def get_vms(vms_file):
    vms = {'localhost': {'ip': '127.0.0.1'}}
    for vf in glob.glob(vms_file):
        with open(vf, 'r') as f:
            lines = f.readlines()
            for line in lines:
                line = line.strip()
                if line and not line.startswith('#'):
                    words = line.split(' => ')
                    vms[words[0]] = {'ip': words[1].split(' ')[0].split(',')[0]}
    return vms

# Reads the network configuration from a file. The VMs in the network will be configured in 
# the same order as they are mentioned in the file.
# Arguments:
#   network_file (string): Path to the file with network configuration
#   vms (dict): VM configurations
#
# Returns:
#   a list of VMs with their configuration details and associated dockers
def get_network(network_file, vms):
    network = []
    with open(network_file, 'r') as f:
        lines = f.readlines()
        for line in lines:
            line = line.strip()
            if line and not line.startswith('#'):
                words = line.split(' => ')
                network.append({'name': words[0], 'ip': vms[words[0]]['ip'], 'dockers': words[1]})
    return network

if __name__ == '__main__':
    SCRIPT_DIR = os.path.dirname(os.path.abspath(sys.argv[0]))

    parser = argparse.ArgumentParser(description='Start Hyperledger fabric on multiple VMs. Must run this program from a VM.')
    parser.add_argument('--fabric-version', type=str, dest='fabric_version', action='store', default='1.4.5',
                        help='Fabric version to use when launching the network.')
    parser.add_argument('--fabric-ca-version', type=str, dest='fabric_ca_version', action='store', default='1.4.5',
                        help='Fabric CA version to use when launching the network.')
    parser.add_argument('--fabric-baseimage-version', type=str, dest='fabric_baseimage_version', action='store', default='0.4.18',
                        help='Fabric baseimage version to use when launching the network.')
    parser.add_argument('--host-username', type=str, dest='host_username', action='store', default='harisj',
                        help='Username in the host where VMs are running.')
    parser.add_argument('--host-hostname', type=str, dest='host_hostname', action='store', default='xaplab450',
                        help='Host where VMs are running.')
    parser.add_argument('-m', type=str, dest='mode', action='store', required=True,
                        choices = ['start', 'stop', 'restart', 'log'], help='Mode for this program.')
    parser.add_argument('-v', type=str, dest='vms_file', action='store', required=True,
                        help='Path to file where each line defines a VM.')
    parser.add_argument('-n', type=str, dest='network_file', action='store', required=True,
                        help='Path to file where each line is a VM => dockers mapping.')
    parser.add_argument('-d', type=str, dest='docker_compose_file', action='store', required=True,
                        help='Path to docker-compose file where dockers are defined. This file is passed on to the VMs, so its path must be valid inside the VMs.')
    parser.add_argument('-o', type=str, dest='output_dir', action='store', help='Path to output directory.')
    args = parser.parse_args()
    FABRIC_VERSION = args.fabric_version
    FABRIC_CA_VERSION = args.fabric_ca_version
    FABRIC_BASEIMAGE_VERSION = args.fabric_baseimage_version
    HOST_USERNAME = args.host_username
    HOST_HOSTNAME = args.host_hostname

    vms = get_vms(args.vms_file)
    network = get_network(args.network_file, vms)

    port = 22
    docker_swarm = set()
    docker_swarm_join_cmd = ''
    docker_compose_file = os.path.abspath(args.docker_compose_file)
    for i, vm in enumerate(network):
        vm_name = vm['name']

        if args.mode == "stop" or args.mode == "restart":
            stop_vm(vm_name, vm['ip'], port)

        if args.mode == "log":
            log_dir = os.path.dirname(docker_compose_file)
            if args.output_dir and i == 0:
                log_dir = os.path.join(log_dir, args.output_dir)
                os.mkdir(log_dir)
            get_docker_logs(vm_name, vm['ip'], port, log_dir, vm['dockers'])

    for i, vm in enumerate(network):
        vm_name = vm['name']

        if args.mode == "start" or args.mode == "restart":
            # Let's consider the first VM as master (which will create the docker swarm) and the
            # other VMs as slaves (which will join the docker swarm).
            if i == 0:
                docker_swarm_join_cmd = start_master_vm(vm_name, vm['ip'], port, docker_compose_file, vm['dockers'])
            else:
                # Check whether the VM has already joined the docker swarm or not.
                join_cmd = '' if vm_name in docker_swarm else docker_swarm_join_cmd
                start_slave_vm(vm_name, vm['ip'], port, join_cmd, docker_compose_file, vm['dockers'])
            docker_swarm.add(vm_name)
