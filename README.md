# Fabric Machine
This project introduces Fabric Machine, an FPGA-based hardware accelerator for Hyperledger Fabric. It is based upon a more general "Blockchain Machine" hardware acceleration technology, which may be extended beyond Fabric to other Hyperledger projects in the future.

## Introduction
This project explores the use of network-attached hardware acceleration for Hyperledger Fabric to improve its performance beyond what is achievable by software-only implementation on a multi-core server. It leverages FPGA accelerator cards (such as Xilinx Alveo), which are being increasingly adopted for accelerating cloud workloads and are also available from major public cloud providers such as AWS and Microsoft Azure.

The scalability and peak performance of Fabric is primarily limited by the bottlenecks present in its block validation/commit phase. The validation phase is run by either an endorser peer (which also endorses transactions) or validator peer. This repo provides an implementation of Fabric Machine, a hardware/software co-designed platform with hardware accelerator and modified Fabric software to act as the hardware-accelerated validator peer in a Fabric network.

The Fabric Machine peer is targeted for a server with a network-attached FPGA card in contrast to existing validator peers which run Fabric software on just a multi-core server. The Fabric Machine peer receives blocks from the orderer through a hardware-friendly protocol, and the block data is retrieved in FPGA without any involvement of the host CPU. The extracted block and its transactions are then passed through an efficient block-level and transaction-level pipeline in FPGA, which implements the bottleneck operations of the validation phase. Finally, Fabric software running on the host CPU accesses the block validation results from hardware, and then commits the block to disk-based ledger just like the software-only validator peer. Overall, a Fabric Machine peer is a hardware/software co-designed peer, leveraging both CPUs and FPGA-based acceleration to deliver significantly better performance than just using CPUs in a multi-core server.

For more details about Blockchain/Fabric Machine architecture, see the publications section [below](#publications).

This repo provides a proof-of-concept implementation and is not meant for production use. The main goal is to engage the Hyperledger community in FPGA-based hardware acceleration, and to refine/improve the Fabric Machine peer based on community experience and feedback.

## How to Use
### Machine Setup
We use a VM to test this repo because it provides a clean environment installed with appropriate tools and dependencies. If you do not want to use a VM, then check out the scripts under ``scripts/vm/`` directory to install the prerequisites manually. Do check those scripts before running them in your environment as they need sudo access and modify system files.

We have provided a vagrant/libvirt based VM setup. If you do not want to use vagrant, then create your own libvirt based VM using one of the online tutorials, e.g., [tutorial1](https://fabianlee.org/2020/02/23/kvm-testing-cloud-init-locally-using-kvm-for-an-ubuntu-cloud-image/) and its [repo](https://github.com/fabianlee/local-kvm-cloudimage), and [tutorial2](https://medium.com/@art.vasilyev/use-ubuntu-cloud-image-with-kvm-1f28c19f82f8). Our VM is created with the following environment:

- OS: Ubuntu 18.04 LTS
- Directories:
    - /opt/go
    - /opt/gopath

To create the VM, first clone this repo and then follow the commands below:
```
cd scripts/vm

# Install vagrant and libvirt/kvm
./install_vagrant_kvm.sh

# Create VM
vagrant up fm-vm

# Login to VM
vagrant ssh fm-vm
```

### Repo Setup
**NOTE:** All the following commands assume that they are run within the VM created in the previous section.

Clone the original Hyperledger Fabric repo, and then clone and merge this repo into the original one.
```
mkdir -p /opt/gopath/src/github.com/hyperledger
cd /opt/gopath/src/github.com/hyperledger
git clone https://github.com/hyperledger/fabric.git
cd fabric && git checkout 11ff991 && cd ..
git clone https://github.com/hyperledger-labs/fabric-machine.git
cp -rf fabric-machine/* fabric/.
cd fabric
```
 
To install the prerequisites, run the following from the ``fabric`` directory. Afterwards, reopen the terminal for the environment variables to take effect:
```
cd scripts/vm
./prereqs.sh
```

To compile the Fabric code and create dockers, run the following from the ``fabric`` directory:
```
# One-time setup (assumes Go paths as mentioned above)
dep ensure
go get -u github.com/golang/dep

# Ignore errors from make *-clean commands which are reported when there are no docker images to clean
# orderer image
make orderer-docker-clean && make orderer-docker

# peer related images
make peer-docker-clean && make peer-docker
```

### Smallbank Benchmark (Software-only Setup)
The software-only setup of smallbank benchmark (from Caliper benchmarks) creates a Docker-based Fabric network on the localhost. The Fabric Machine peer is simulated/emulated in software, without real deployment on an FPGA card. The goal is to provide an easy setup and testing of Fabric Machine peer, and obtain quick estimate of speedup in block validation due to hardware acceleration.

To run the benchmark (ignore errors from docker creation/shutdown commands):
```
cd samples/smallbank
./setup_benchmark.sh  # one-time setup
./run_benchmark.sh
```

### Expected Output
The script will print statistics for each of the peers in Fabric network. The most important metric reported is the commit latency/throughput with and without ledger_write operation. Since ledger_write operation is executed in software in both vanilla Fabric peer and Fabric Machine peer, a direct comparison should be between commit latency/throughput without ledger_write operation. For example, in the sample output below, we observe a huge improvement in commit throughput with Fabric Machine peer (1,019 --> 18,220 tps). 

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

## Publications
- [[Talk](https://wiki.hyperledger.org/display/PSWG/PSWG+July+27%2C+2021)] H. Javaid. 2021. _Blockchain Machine: Accelerating Validation Bottlenecks in Hyperledger Fabric_. Hyperledger Performance and Scale Working Group.
- [[Paper](http://arxiv.org/abs/2104.06968)] H. Javaid, J. Yang, N. Santoso, M. Upadhyay, S. Mohan, C. Hu, G. Brebner. 2021. _Blockchain Machine: A Network-Attached Hardware Accelerator for Hyperledger Fabric_. arXiv:2104.06968.
- [[Talk](https://www.youtube.com/watch?v=GoOYO_ju7mA)] H. Javaid. 2020. _Hyperledger Performance Improvements (Presentation, Demo and Discussion)_. Hyperledger Sydney Meetup.
- [[Talk](https://www.youtube.com/watch?v=Nidw6zMR4hs)] S. Mohan. 2020. _Hyperledger Performance Improvements (Demo and Discussion)_. Hyperledger San Francisco Meetup.

## Initial Committers
- Ji Yang, Xilinx (https://github.com/yangji-xlnx)
- Haris Javaid, Xilinx (https://github.com/harisj-xlnx)

## Sponsors
- Mark Wagner, Red Hat (mwagner@redhat.com)
- Vipin Bharathan, Hyperledger Labs Steward (vip@dlt.nyc)
- David Boswell, Director of Ecosystem at Linux Foundation (dboswell@linuxfoundation.org)
