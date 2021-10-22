# Fabric Machine
This project introduces Fabric Machine, an FPGA-based hardware accelerator for Hyperledger Fabric. It is based upon a more general "Blockchain Machine" hardware acceleration technology, which may be extended beyond Fabric to other Hyperledger projects in the future.

## Introduction
This project explores the use of network-attached hardware acceleration for Hyperledger Fabric to improve its performance beyond what is achievable by software-only implementation on a multi-core server. It leverages FPGA accelerator cards (such as Xilinx Alveo), which are being increasingly adopted for accelerating cloud workloads and are also available from major public cloud providers such as AWS and Microsoft Azure.

The scalability and peak performance of Fabric is primarily limited by the bottlenecks present in its block validation/commit phase. The validation phase is run by either an endorser peer (which also endorses transactions) or validator peer. This repo provides an implementation of Fabric Machine, a hardware/software co-designed platform with hardware accelerator and modified Fabric software to act as the hardware-accelerated validator peer in a Fabric network.

The Fabric Machine peer is targeted for a server with a network-attached FPGA card in contrast to existing validator peers which run Fabric software on just a multi-core server. The Fabric Machine peer receives blocks from the orderer through a hardware-friendly protocol, and the block data is retrieved in FPGA without any involvement of the host CPU. The extracted block and its transactions are then passed through an efficient block-level and transaction-level pipeline in FPGA, which implements the bottleneck operations of the validation phase. Finally, Fabric software running on the host CPU accesses the block validation results from hardware, and then commits the block to disk-based ledger just like the software-only validator peer. Overall, a Fabric Machine peer is a hardware/software co-designed peer, leveraging both CPUs and FPGA-based acceleration to deliver significantly better performance than just using CPUs in a multi-core server.

For more details about Blockchain/Fabric Machine architecture, see the publications section [below](#publications).

This repo provides a proof-of-concept implementation and is not meant for production use. The main goal is to engage the Hyperledger community in FPGA-based hardware acceleration, and to refine/improve the Fabric Machine peer based on community experience and feedback.

## How to Use
### Repo Setup
Clone the original Hyperledger Fabric [repo](https://github.com/hyperledger/fabric), and then clone and merge this repo into the original one.

```
git clone https://github.com/hyperledger/fabric.git
cd fabric && git checkout 11ff991 && cd ..
git clone https://github.com/Xilinx/hyperledger-fabric.git
cp -rf hyperledger-fabric/* fabric/.
cd fabric
```
 
To install the prerequisites, run the following from the ``fabric`` directory. Afterwards, reopen the terminal:
```
cd scripts
./prereqs.sh  # needs sudo access
```

To compile the Fabric code and create dockers, run the following from the ``fabric`` directory:
```
# One-time setup
dep ensure
go get -u github.com/golang/dep

# orderer image
make orderer-docker-clean && make orderer-docker

# peer related images
make ccenv-docker-clean && make ccenv
make peer-docker-clean && make peer-docker
```

### Smallbank Benchmark (Software-only Setup)
The software-only setup of smallbank benchmark (from Caliper benchmarks) creates a Docker-based Fabric network on the localhost. The Fabric Machine peer is simulated/emulated in software, without real deployment on an FPGA card. The goal is to provide an easy setup and testing of Fabric Machine peer, and obtain quick estimate of speedup in block validation due to hardware acceleration.

To run the benchmark:
```
cd samples/smallbank
./setup_benchmark.sh  # one-time setup
./run_benchmark.sh
```

_Stay tuned for more updates soon!_

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
