# Fabric Machine
This repo provides a proof-of-concept implementation and is not meant for production use. The main goal is to engage the Hyperledger community in FPGA-based hardware acceleration, and to refine/improve the Fabric Machine peer based on community experience and feedback.

# How to Use
## Machine Setup
We use a VM to test this repo because it provides a clean environment installed with appropriate tools and dependencies. If you do not want to use a VM, then check out the scripts under ``scripts/vm/`` directory to install the prerequisites manually. Do check those scripts before running them in your environment as they need sudo access and modify system files.

We have provided a vagrant/libvirt based VM setup. If you do not want to use vagrant, then create your own libvirt based VM using one of the online tutorials, e.g., [tutorial1](https://fabianlee.org/2020/02/23/kvm-testing-cloud-init-locally-using-kvm-for-an-ubuntu-cloud-image/) and its [repo](https://github.com/fabianlee/local-kvm-cloudimage), and [tutorial2](https://medium.com/@art.vasilyev/use-ubuntu-cloud-image-with-kvm-1f28c19f82f8). Our VM is created with the following environment:

- OS: Ubuntu 18.04 LTS
- Directories:
    - /opt/go
    - /opt/gopath

To create the VM, first clone this repo and then follow the commands below:
```
cd v1.4/scripts/vm

# Install vagrant and libvirt/kvm
./install_vagrant_kvm.sh

# Create VM
vagrant up fm-vm

# Login to VM
vagrant ssh fm-vm
```

## Repo Setup
**NOTE:** All the following commands assume that they are run within the VM created in the previous section.

Clone the original Hyperledger Fabric repo, and then clone and merge this repo into the original one.
```
mkdir -p /opt/gopath/src/github.com/hyperledger
cd /opt/gopath/src/github.com/hyperledger

git clone https://github.com/hyperledger/fabric.git
git clone https://github.com/hyperledger-labs/fabric-machine.git
./fabric-machine/v1.4/scripts/fabric/fabric-repo.sh -d fabric/ -s

cd fabric
```

To install the prerequisites, run the following from the ``fabric`` directory. Afterwards, reopen the terminal for the environment variables to take effect:
```
cd scripts/vm
./prereqs.sh
```

To compile the Fabric code and create dockers, run the following from the ``fabric`` directory:
```
# Ignore errors from make *-clean commands which are reported when there are no docker images to clean
# orderer image
make orderer-docker-clean && make orderer-docker

# peer related images
make peer-docker-clean && make peer-docker

# One-time setup (used by peer during installation of chaincode)
docker pull hyperledger/fabric-ccenv:1.4.5 && docker tag hyperledger/fabric-ccenv:1.4.5 hyperledger/fabric-ccenv:latest

```

## Smallbank Benchmark (Software-only Setup)
The software-only setup of smallbank benchmark (from Caliper benchmarks) creates a Docker-based Fabric network on the localhost. The Fabric Machine peer is simulated/emulated in software, without real deployment on an FPGA card. The goal is to provide an easy setup and testing of Fabric Machine peer, and obtain quick estimate of speedup in block validation due to hardware acceleration.

To run the benchmark (ignore errors from docker creation/shutdown commands):
```
cd samples/smallbank
./setup_benchmark.sh  # one-time setup
./run_benchmark.sh
```

If you want to set up the benchmark from scratch by cleaning the working directory, then use:
```
./clean_benchmark.sh -l
./setup_benchamrk.sh
```

## Expected Output
The script will print statistics for each of the peers in Fabric network. The most important metric reported is the commit latency/throughput with and without ledger_write operation. Since ledger_write operation is executed in software in both vanilla Fabric peer and Fabric Machine peer, a direct comparison should be between commit latency/throughput without ledger_write operation. For example, in the sample output below, we observe a huge improvement in commit throughput with Fabric Machine peer (18,220 tps vs 1,019 tps).

```
INFO: peer0.org1.example.com -- transactions (succeeded/total) = 1830/2028
INFO:      with ledger_write -- commit latency (ms) = 165.155 commit throughput (tps) = 558
INFO:   without ledger_write -- commit latency (ms) = 90.46 commit throughput (tps) = 1019

INFO: peer2.org1.example.com -- transactions (succeeded/total) = 1830/2028
INFO:      with ledger_write -- commit latency (ms) = 52.335 commit throughput (tps) = 1767
INFO:   without ledger_write -- commit latency (ms) = 5.078 commit throughput (tps) = 18220
```

The commit latency/throughput with ledger_write executes the ledger_write operation sequentially after validation, hence slowing down the entire peer. However, the ledger_write operation can be run asynchronously either in the same peer node or on a separate storage node [1], but such an optimization is not yet implemented in Fabric Machine peer.

[1] C. Gorenflo et al. 2019. _FastFabric: Scaling Hyperledger Fabric to 20,000 Transactions per Second_. IEEE International Conference on Blockchain and Cryptocurrency (ICBC).

## Smallbank Benchmark (Hardware/Software Setup)
To setup a Fabric Machine peer, you will need the following hardware setup:
- Server1 with 100G NIC card
- Server2 with Xilinx Alveo U250 FPGA card
- A direct 100G connection between server1 and server2, or the connection can be through a 100G switch (use port0 of Alveo U250 card which is the top QSFP port away from the PCIe slot)
- A USB/JTAG cable from server1 to Alveo U250 FPGA card (used for programming the FPGA card)

The FPGA card in server2 needs to be programmed with a bitstream generated from Fabric Machine hardware. Server1 will run the orderer while server2 will run the Fabric Machine peer, so server1 will send blocks to server2.

Please reach out to us for the bitstream and more details on/help with this setup. We plan to make this setup available in [Heterogeneous Accelerated Compute Cluster (HACC) at National University of Singapore (NUS)](https://xilinx.github.io/xacc/nus.html) so that the community can try Fabric Machine peer with ease. Stay tuned for more updates!
