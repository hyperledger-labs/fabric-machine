/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package bcm

import (
	"bytes"
	"encoding/binary"
	_ "encoding/hex"
	"fmt"
	"math"
)


type MessageType int

const (
	CacheSync = iota
	BlockHeader
	BlockTx
	BlockMeta
	BlockACK
	BlockNAK
	CacheACK
	CacheNAK
)

type BcmBlockReply struct {
	t         byte
	block_num uint32
	tx_num    int16
}

func blockReplyToBytes(reply BcmBlockReply) (data []byte) {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, reply)
	data = buf.Bytes()
	return data
}

type BcmTransportHeader struct {
	Sequence  uint16
	Ctrl_type uint8
	//annotation_size uint8
}

func (hdr BcmTransportHeader) ToBytes() (data []byte) {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, hdr)
	data = buf.Bytes()
	return data
}

type BcmProtocol struct {
	// Send Status
	SenderBlockNumber int64
	SenderTxNumber    int
	// Receiver Status
	ReceiverBlockNumber int64
	ReceiverTxNumber    int
	// Receiver Cache Effective Window
	ReceiverWndFromBlkNum int64
	ReceiverWndFromTxNum  int
	ReceiverWndToBlkNum   int64
	ReceiverWndToTxNum    int
	// Receiver Runtime
	Block_tx_size int
	// Receiver Cache Abstraction
	ReceiverCacheAbstraction uint64

	// FIXME: move identity cache here

	// Sessions
	blockHeaderStrm BcmStream // TODO: the 3 block stream can be merged in to 1
	blockTxStrm     BcmStream
	blockMetaStrm   BcmStream
	controlStrm     BcmStream
	// FIXME: dbSyncStrm gossipSession

	// Connection
	remotePeer BcmConnection


	blockBuffer []byte
}

func ConnectToPeer(localAddr string, remoteAddr string, cfgFileFolder string) *BcmProtocol {
	err := readConfig(cfgFileFolder)
	if err == -1 {
		return nil
	}
	//conn := newDCNLightUDPConn("0.0.0.0:49656", "127.0.0.1:49656")
	conn := newDCNLightUDPConn(localAddr, remoteAddr)
	conn.StartListen()
	blkHdrStrm := conn.OpenStream(0x1)
	blkTxStrm := conn.OpenStream(0x2)
	blkMetaStrm := conn.OpenStream(0x3)
	controlStrm := conn.OpenStream(0x0)
	return &BcmProtocol{0, 0, -1, -1, -1, -1, -1, -1, 0, 0, blkHdrStrm, blkTxStrm, blkMetaStrm, controlStrm, conn, []byte{}}
}

func (bmp *BcmProtocol) SendBlock(blkNum int64, data []byte) {
	if blkNum < bmp.ReceiverWndFromBlkNum || blkNum > bmp.ReceiverWndFromBlkNum {
		// TODO: push to send buffer, trigger cache update, trigger wait send
		bmp.UpdateRemoteCache(blkNum)
		bmp.GetRemoteStatus()
		bmp.SendOutBlock(data)
	} else {
		bmp.SendOutBlock(data)
	}
	bmp.SenderBlockNumber = blkNum + 1
}

// TODO:
func (bmp *BcmProtocol) AcceptBlock() int {
	return 0
}

// TODO:
func (bmp *BcmProtocol) UpdateRemoteCache(targetBlkNum int64) {
	bmp.FlushCertificateEntriesToPeer()
	bmp.ReceiverWndFromBlkNum = 1
	bmp.ReceiverWndFromTxNum = 0
	bmp.ReceiverWndToBlkNum = math.MaxInt64
	bmp.ReceiverWndToTxNum = 256
}

// TODO:
func (bmp *BcmProtocol) GetRemoteStatus() {
	bmp.ReceiverBlockNumber = 1
	bmp.ReceiverTxNumber = 0
}

func (bmp *BcmProtocol) SendOutBlock(data []byte) {
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
	logger.Infof("Send block header")
	bmp.SendBlockHeader(data, BlockHeaderPos, BlockHeaderLen, TransactionLengthList)
	for i := 0; i < len(TransactionPosList); i++ {
		logger.Infof("Send transaction")
		bmp.SendTransaction(data, TransactionPosList[i], TransactionLengthList[i])
	}
	logger.Infof("Send metadata")
	bmp.SendBlockMeta(data, BlockMetaPos, BlockMetaLen)
}

// SendBlockHeader prepares packet data based on block header and transaction information
// then send to target address
func (bmp *BcmProtocol) SendBlockHeader(data []byte, pos int, length int, transactionLen []int) {
	annotation := generateBlockAnnotation(pos, length, transactionLen, data)
	payload := data[pos : pos+length]
	bmp.blockHeaderStrm.Write([]byte{byte(len(annotation))}, annotationListToBytes(annotation), payload)
}

// SendTransaction prepares packet data based on transaction and send to target address
func (bmp *BcmProtocol) SendTransaction(data []byte, pos int, length int) {
	annotation := generateTransactionAnnotation(pos, length, data)
	payload := adjustDataBasedOnLocator(data, pos, length, annotation)
	bmp.blockTxStrm.Write([]byte{byte(getAnnotationActiveSize(annotation))}, annotationListToBytes(annotation), payload)
}

// SendBlockMeta prepares packet data based on block metadata and send to target address
func (bmp *BcmProtocol) SendBlockMeta(data []byte, pos int, length int) {
	annotation := generateBlockMetaAnnotation(pos, length, data)
	payload := adjustDataBasedOnLocator(data, pos, length, annotation)
	bmp.blockMetaStrm.Write([]byte{byte(len(annotation))}, annotationListToBytes(annotation), payload)
}

// SendCertificateEntryUpdate
func (bmp *BcmProtocol) SendCertificateEntryUpdate(id int, name string, ca []byte) {
	payload, annotation := generateCertificateUpdateAnnotation(id, name, ca)
	bmp.controlStrm.Write([]byte{byte(len(annotation))}, annotationListToBytes(annotation), payload)
}

func (bmp *BcmProtocol) FlushCertificateEntriesToPeer() {
	cacheImage := getCertificateCache()
	for _, v := range cacheImage {
		bmp.SendCertificateEntryUpdate(v.id, v.name, v.ca)
	}
}
type BcmHeader struct {
	TransportHdr BcmTransportHeader
	AnnotationCount	uint8
}

func (bmp *BcmProtocol) rawPacketProcess(data []byte) (dType byte, annotations []Annotation, blockPayload []byte) {
	buf := bytes.NewBuffer(data)
	hdr := &BcmHeader{}
	binary.Read(buf, binary.BigEndian, hdr)
	dType = hdr.TransportHdr.Ctrl_type&0x0F
	phyAnnotationValue := int(hdr.AnnotationCount)
	if dType == 0x02 {
		phyAnnotationValue = phyAnnotationValue + 6
	}
	for i:=0; i<phyAnnotationValue; i++ {
		a := &Annotation{}
		binary.Read(buf, binary.BigEndian, a)
		annotations = append(annotations, *a)
	}
	dType = hdr.TransportHdr.Ctrl_type&0x0F
	//fmt.Println("annotation size:", len(annotations))
	blockPayload = recoverDataBasedOnLocator(buf.Bytes(), annotations)
	//fmt.Println(hex.Dump(blockPayload))
	return dType, annotations, blockPayload
}

func (bmp *BcmProtocol) BlockRecvHeader(data []byte) {
	bmp.Block_tx_size = 1 // FIXME
	_,_,payload := bmp.rawPacketProcess(data)
	bmp.WriteBlockBuffer(payload)
	bmp.BlockReceiveControlSM(BlockHeader, 1, -1)
}

func (bmp *BcmProtocol) BlockRecvTx(data []byte) {
	_,_,payload := bmp.rawPacketProcess(data)
	bmp.WriteBlockBuffer(payload)
	bmp.BlockReceiveControlSM(BlockTx, 1, 0)
}

func (bmp *BcmProtocol) BlockRecvMeta(data []byte) {
	_,_,payload := bmp.rawPacketProcess(data)
	bmp.WriteBlockBuffer(payload)
	bmp.BlockReceiveControlSM(BlockMeta, 1, bmp.Block_tx_size)
}

func (bmp *BcmProtocol) BlockReceiveControlSM(t MessageType, block_num int, tx_num int) { // msg_type
	if int64(block_num) == bmp.ReceiverBlockNumber && tx_num == bmp.ReceiverTxNumber { // correct status
		fmt.Println("Accept new comming data...")
		fmt.Println("Move status.")
		msg := BcmBlockReply{BlockACK, uint32(bmp.ReceiverBlockNumber), int16(bmp.ReceiverTxNumber)}
		bmp.controlStrm.Write(blockReplyToBytes(msg))
		// move to next
		if tx_num == bmp.Block_tx_size {
			bmp.ReceiverBlockNumber++
			bmp.ReceiverTxNumber = -1
			bmp.Block_tx_size = 0
		} else {
			bmp.ReceiverTxNumber++
		}
	} else {
		// Too old
		msg := BcmBlockReply{BlockACK, uint32(bmp.ReceiverBlockNumber), int16(bmp.ReceiverTxNumber)}
		bmp.controlStrm.Write(blockReplyToBytes(msg))
		// Too new ??
	}
}


func (bmp *BcmProtocol) WriteBlockBuffer(msg []byte) {
	bmp.blockBuffer = append(bmp.blockBuffer, msg...)
}

func (bmp *BcmProtocol) GetBlockBuffer() ([]byte) {
	return bmp.blockBuffer
}

func (bmp *BcmProtocol) ClearBlockBuffer() {
	bmp.blockBuffer = bmp.blockBuffer[:0]
}

/*
func (bmp *BcmProtocol) BlockSendControlSM(data []byte) {

}

func (bmp *BcmProtocol) CacheReceiveControlSM() {

}

func (bmp *BcmProtocol) CacheSendControlSM(type byte, curr_blk_num int, curr_tx_num int) {

}
*/
