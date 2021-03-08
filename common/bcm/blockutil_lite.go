/*
Copyright IBM Corp. All Rights Reserved.
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package bcm

import (
	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/peer"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
)

// UnmarshalPayload unmarshals bytes to a Payload
// hyperledger/fabric@v2.0/protoutil/unmarshalers.go
func UnmarshalPayload(encoded []byte) (*cb.Payload, error) {
	payload := &cb.Payload{}
	err := proto.Unmarshal(encoded, payload)
	return payload, errors.Wrap(err, "error unmarshaling Payload")
}

// UnmarshalChannelHeader unmarshals bytes to a ChannelHeader
// hyperledger/fabric@v2.0/protoutil/unmarshalers.go
func UnmarshalChannelHeader(bytes []byte) (*cb.ChannelHeader, error) {
	chdr := &cb.ChannelHeader{}
	err := proto.Unmarshal(bytes, chdr)
	return chdr, errors.Wrap(err, "error unmarshaling ChannelHeader")
}

// UnmarshalChaincodeHeaderExtension unmarshals bytes to a ChaincodeHeaderExtension
// hyperledger/fabric@v2.0/protoutil/unmarshalers.go
func UnmarshalChaincodeHeaderExtension(hdrExtension []byte) (*peer.ChaincodeHeaderExtension, error) {
	chaincodeHdrExt := &peer.ChaincodeHeaderExtension{}
	err := proto.Unmarshal(hdrExtension, chaincodeHdrExt)
	return chaincodeHdrExt, errors.Wrap(err, "error unmarshaling ChaincodeHeaderExtension")
}

// UnmarshalChaincodeID unmarshals bytes to a ChaincodeID
// hyperledger/fabric@v2.0/protoutil/unmarshalers.go
func UnmarshalChaincodeID(bytes []byte) (*peer.ChaincodeID, error) {
	ccid := &pb.ChaincodeID{}
	err := proto.Unmarshal(bytes, ccid)
	return ccid, errors.Wrap(err, "error unmarshaling ChaincodeID")
}

// GetEnvelopeFromBlock gets an envelope from a block's Data field.
// hyperledger/fabric@v2.0/protoutil/txutils.go
func GetEnvelopeFromBlock(data []byte) (*cb.Envelope, error) {
	// Block always begins with an envelope
	var err error
	env := &cb.Envelope{}
	if err = proto.Unmarshal(data, env); err != nil {
		return nil, errors.Wrap(err, "error unmarshaling Envelope")
	}

	return env, nil
}

// ExtractEnvelope retrieves the requested envelope from a given block and
// unmarshals it
// hyperledger/fabric@v2.0/protoutil/commonutils.go
func ExtractEnvelope(block *cb.Block, index int) (*cb.Envelope, error) {
	if block.Data == nil {
		return nil, errors.New("block data is nil")
	}

	envelopeCount := len(block.Data.Data)
	if index < 0 || index >= envelopeCount {
		return nil, errors.New("envelope index out of bounds")
	}
	marshaledEnvelope := block.Data.Data[index]
	envelope, err := GetEnvelopeFromBlock(marshaledEnvelope)
	//err = errors.WithMessagef(err, "block data does not carry an envelope at index %d", index)
	return envelope, err
}

// IsConfigBlock validates whenever given block contains configuration
// update transaction
// hyperledger/fabric@v2.0/protoutil/commonutils.go
func IsConfigBlock(block *cb.Block) bool {
	envelope, err := ExtractEnvelope(block, 0)
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

	hdr, err := UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return false
	}

	return cb.HeaderType(hdr.Type) == cb.HeaderType_CONFIG || cb.HeaderType(hdr.Type) == cb.HeaderType_ORDERER_TRANSACTION
}
