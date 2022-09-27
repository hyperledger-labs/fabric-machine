/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fmprotocol

import (
	"context"
	"strings"
	"sync"

	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/fabricmachine/api"
	"google.golang.org/grpc/metadata"
)

var logger = flogging.MustGetLogger("fmprotocol")

// Keeps hardware peer related information.
type HardwarePeer struct {
	sync.RWMutex
	initDone bool

	// Address of the hardware peer.
	address string

	// True when the current node is considered an orderer node that can send blocks.
	isOrderer bool

	// Next block that should be sent.
	blockToSend uint64
}

var hwPeer HardwarePeer

// isOrdererNode returns true when the provided node name/id is considered an orderer node that
// can send blocks to hardware peer.
func isOrderer(node string) bool {
	if !fmapi.IsEnabled() {
		return false
	}

	for _, o := range fmapi.GetOrderers() {
		if o == node {
			return true
		}
	}
	return false
}

func InitConfig() {
	if !fmapi.IsEnabled() {
		return
	}

	hwPeer.address = fmapi.GetHardwareAddress()
	hwPeer.blockToSend = fmapi.GetStartingBlock()
	logger.Info("Initialized Fabric machine protocol with initial configuration.")
}

func completeInit(ctx context.Context) {
	// Update isOrderer flag.
	// Name of the current node ignoring port number.
	grpcInMd, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.Warning("Could not extract node name/id from context")
		return
	}
	thisNode := grpcInMd.Get(":authority")[0]
	thisNode = strings.Split(thisNode, ":")[0]
	hwPeer.isOrderer = isOrderer(thisNode)

	// Update certificates cache in hardware peer.
	if hwPeer.isOrderer {
		logger.Infof("Loading certificates from hardware config file ...")
		initCertificateCache()
		readConfig()
		logger.Infof("Syncing certificates with hardware peer ...")
		updateRemoteCertificateCache(hwPeer.address)
	}

	hwPeer.initDone = true
	logger.Info("Completed Fabric machine protocol configuration.")
}

// CheckPeerStructure checks if peer is a hardware based peer
func CheckPeerStructure(addr string) (ret bool) {
	// TODO: check if the peer is a hardware based peer
	return true
}

// CheckMessageData checks block message format, return false if the block is not supported
func CheckMessageData(block *cb.Block) (ret bool) {
	if block.Header.Number == 0 {
		logger.Warningf("Wrong block number %d", block.Header.Number)
		return false
	}
	config, err := IsConfigBlock(block)
	if err != nil {
		logger.Warningf("Cannot check config block: %s", err)
		return false
	}
	if config {
		logger.Warning("Got a config block")
		return false
	}
	envelope, err := GetEnvelopeFromBlock(block.Data.Data[0])
	if err != nil {
		logger.Warningf("Cannot get block.Data.Data[0] envelope: %s", err)
		return false
	}
	payload, err := UnmarshalPayload(envelope.Payload)
	if err != nil {
		logger.Warningf("Cannot unmarshal envelope payload: %s", err)
		return false
	}
	if payload.Header == nil {
		logger.Warningf("Missing header in envelope payload")
		return false
	}
	chdr, err := UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		logger.Warningf("Cannot unmarshal channel header: %s", err)
		return false
	}
	if len(chdr.Extension) == 0 {
		logger.Warningf("Missing extension in channel header")
		return false
	}
	hdrExt, err := UnmarshalChaincodeHeaderExtension(chdr.Extension)
	if err != nil {
		logger.Warningf("Cannot unmarshal chaincode header: %s", err)
		return false
	}
	if hdrExt.ChaincodeId == nil {
		logger.Warningf("Missing chaincode id")
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
	if !fmapi.IsEnabled() {
		return
	}

	hwPeer.Lock()
	defer hwPeer.Unlock()

	// Complete initialization when this function is called for the first time.
	if !hwPeer.initDone {
		completeInit(ctx)
	}

	// Only orderer will send blocks to hardware peer.
	if !hwPeer.isOrderer {
		return
	}

	// Make sure that a block is only sent once.
	if block.Header.Number != hwPeer.blockToSend {
		logger.Warningf("Expected to send block %d to hardware peer but received block %d\n", hwPeer.blockToSend, block.Header.Number)
		return
	}

	// Check peer compatibility.
	addr := hwPeer.address
	isHardware := CheckPeerStructure(addr)
	if !isHardware {
		return
	}

	// Make sure there are no configuration update blocks, otherwise CheckMessageData() below will
	// fail and this function will be stuck without sending any blocks.
	// TODO: Update CheckMessageData to differentiate between normal and configuration update blocks
	// so that configuration update blocks can be skipped.
	isBlockData := CheckMessageData(block)
	if isBlockData == false {
		logger.Warningf("Received ill-formed block %d\n", block.Header.Number)
		return
	}

	// Send block.
	logger.Infof("Sending block %d to hardware peer %s\n", block.Header.Number, addr)
	SendBlock(addr, block)
	hwPeer.blockToSend++
}
