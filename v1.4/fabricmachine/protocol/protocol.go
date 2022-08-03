/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fmprotocol

import (
	"context"
	"strings"
	"sync"

	"github.com/hyperledger/fabric/common/flogging"
	cb "github.com/hyperledger/fabric/protos/common"
	"google.golang.org/grpc/metadata"
)

var logger = flogging.MustGetLogger("BM-Protocol")

// Keeps hardware peer related information.
type HardwarePeer struct {
	sync.RWMutex
	address  string
	blockNum uint64
}

// We skip the first block (block0 is generis block, block1 is chaincode instantiation block).
var hwPeer = HardwarePeer{
	address:  "192.55.0.54:49656",
	blockNum: 1,
}

// CheckPeerStructure checks if peer is a hardware based peer
func CheckPeerStructure(addr string) (ret bool) {
	// TODO: check if the peer is a hardware based peer
	return true
}

// CheckMessageData checks block message format, return false if the block is not supported
func CheckMessageData(block *cb.Block) (ret bool) {
	if block.Header.Number == 0 {
		return false
	}
	if IsConfigBlock(block) == true {
		return false
	}
	envelope, err := GetEnvelopeFromBlock(block.Data.Data[0])
	if err != nil {
		return false
	}
	payload, err := UnmarshalPayload(envelope.Payload)
	if err != nil {
		return false
	}
	if payload.Header == nil {
		return false
	}
	chdr, err := UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return false
	}
	if len(chdr.Extension) == 0 {
		return false
	}
	hdrExt, err := UnmarshalChaincodeHeaderExtension(chdr.Extension)
	if err != nil {
		return false
	}
	if hdrExt.ChaincodeId == nil {
		return false
	}
	// FIXME: quick fix, disable chaincode check.
	//        should be able to support all chaincode ID
	//if hdrExt.ChaincodeId.Name != "smallbank" {
	//	return false
	//}
	return true
}

// SendToHardware broadcasts a raw block message to hardware based peers
func SendToHardware(ctx context.Context, block *cb.Block) {
	hwPeer.Lock()
	defer hwPeer.Unlock()

	// update cache first
	if block.Header.Number == 0 {
		logger.Infof("Load certificates")
		initCertificateCache()
		read_config()
		logger.Infof("Sync certificate with hw peer")
		updateRemoteCertificateCache(hwPeer.address)
		return
	}

	// Make saure there are no configuration update blocks, otherwise CheckMessageData() below will
	// fail and this function will be stuck here without sending any blocks.
	// This check ensures that a block is only sent once.
	if block.Header.Number != hwPeer.blockNum {
		logger.Warningf("Expected to send block %d to hardware peer but received block %d\n", hwPeer.blockNum, block.Header.Number)
		return
	}

	grpcInMd, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return
	}

	// Name of the current node ignoring port number.
	thisNode := grpcInMd.Get(":authority")[0]
	thisNode = strings.Split(thisNode, ":")[0]
	if thisNode != "orderer.example.com" && thisNode != "orderer0.example.com" { // Only orderer will send blocks to hardware peer.
		return
	}
	addr := hwPeer.address

	// check architecture
	isHardware := CheckPeerStructure(addr)
	if isHardware == false {
		return
	}

	// TODO: Update CheckMessageData to differentiate between normal and configuration update blocks
	// so that configuration update blocks can be skipped.
	isBlockData := CheckMessageData(block)
	if isBlockData == false {
		logger.Warningf("Received ill-formed block %d\n", block.Header.Number)
		return
	}

	// send block
	logger.Infof("Sending block %d to hardware peer %s\n", block.Header.Number, addr)
	SendBlock(addr, block)
	hwPeer.blockNum++
}
