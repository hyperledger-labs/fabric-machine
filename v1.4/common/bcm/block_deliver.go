/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package bcm

import (
	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric/protos/common"
)

// SendBlockHeader prepares packet data based on block header and transaction information
// then send to target addres
func SendBlockHeader(addr string, data []byte, pos int, length int, transactionLen []int) {
	annotation := generateBlockAnnotation(pos, length, transactionLen, data)
	payload := data[pos : pos+length]
	bcmSend(addr, 0x1, annotationListToBytes(annotation), len(annotation), payload)
}

// SendTransaction prepares packet data based on transaction and send to target addres
func SendTransaction(addr string, data []byte, pos int, length int) {
	annotation := generateTransactionAnnotation(pos, length, data)
	payload := adjustDataBasedOnLocator(data, pos, length, annotation)
	bcmSend(addr, 0x2, annotationListToBytes(annotation), getAnnotationActiveSize(annotation), payload)
}

// SendBlockMeta prepares packet data based on block metadata and send to target addres
func SendBlockMeta(addr string, data []byte, pos int, length int) {
	annotation := generateBlockMetaAnnotation(pos, length, data)
	payload := adjustDataBasedOnLocator(data, pos, length, annotation)
	bcmSend(addr, 0x3, annotationListToBytes(annotation), len(annotation), payload)
}

// SendBlock sends a block to target hardware peer via blockchain machine protocol
func SendBlock(addr string, block *cb.Block) {
	data, err := proto.Marshal(block)
	if err != nil {
		logger.Errorf("BCM error: block serilization failed: %v", err)
		return
	}

	// block format: block_header, transaction_1, ..., transaction_N,
	//   block_metadata
	pos := 0
	// block header: fleld == 1, length ~=70
	field, length, pos := protoGetFieldLength(data, pos)
	pos += length

	// block data start(envelope): field == 2
	field, length, pos = protoGetFieldLength(data, pos)

	BlockHeaderPos := 0
	BlockHeaderLen := pos - BlockHeaderPos
	BlockMetaPos := pos + length

	// read transactions
	var TransactionPosList []int = nil
	var TransactionLengthList []int = nil
	for pos < BlockMetaPos {
		TransactionPosList = append(TransactionPosList, pos)
		field, length, pos = protoGetFieldLength(data, pos)
		// field == 1, new transaction, else error ...
		if field != 1 {
			if len(TransactionPosList) > 0 {
				TransactionPosList = TransactionPosList[:len(TransactionPosList)-1]
			}
		} else {
			// ERROR!!!
		}
		pos += length
		TransactionLengthList = append(TransactionLengthList,
			pos-TransactionPosList[len(TransactionPosList)-1])
	}

	// read metadata
	field, length, pos = protoGetFieldLength(data, pos)
	BlockMetaLen := pos + length - BlockMetaPos

	// send data
	logger.Debugf("Send block [%d] header", block.Header.Number)
	SendBlockHeader(addr, data, BlockHeaderPos, BlockHeaderLen, TransactionLengthList)
	for i := 0; i < len(TransactionPosList); i++ {
		logger.Debugf("Send block [%d] tx", block.Header.Number)
		SendTransaction(addr, data, TransactionPosList[i], TransactionLengthList[i])
	}
	logger.Debugf("Send block [%d] metadata", block.Header.Number)
	SendBlockMeta(addr, data, BlockMetaPos, BlockMetaLen)
}

// sendCertificateCacheUpdate sends certifcate cache update message to target hardware peer
// via blockchain machine protocol
func sendCertificateCacheUpdate(addr string, id int, name string, ca []byte) {
	payload, annotations := generateCertificateUpdateAnnotation(id, name, ca)
	bcmSend(addr, 0x0, annotationListToBytes(annotations), len(annotations), payload)
}
