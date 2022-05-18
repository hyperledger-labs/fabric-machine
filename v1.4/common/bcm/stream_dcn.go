/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package bcm

import (
	"sync"
)

type BcmDCNLightUDPStream struct {
	sync.RWMutex
	id   int
	conn *BcmDCNLightUDPConn
	// send
	sequence uint16
	control  uint8
	info     uint32
	// receive
	msgType    byte
	recvBuffer [][]byte
	// control
	timeout int
}

func newDCNLightUDPStream(id int, conn *BcmDCNLightUDPConn, msgTypeListen byte) *BcmDCNLightUDPStream {
	return &BcmDCNLightUDPStream{
		id:         id,
		conn:       conn,
		sequence:   0,
		control:    0,
		info:       0,
		msgType:    msgTypeListen,
		recvBuffer: [][]byte{},
		timeout:    -1,
	}
}

func (stream *BcmDCNLightUDPStream) StreamID() int {
	return stream.id
}

func (stream *BcmDCNLightUDPStream) ReceiveStreamID() int {
	return stream.id
}

func (stream *BcmDCNLightUDPStream) SendStreamID() int {
	return stream.id
}

func (stream *BcmDCNLightUDPStream) generateBcmTransportHeader(msgType byte) BcmTransportHeader {
	return BcmTransportHeader{stream.sequence, uint8(byte(stream.control<<4) | msgType)}
}

func (stream *BcmDCNLightUDPStream) Write(dataSlices ...[]byte) (int, error) {
	// TODO: Write based on msgType
	bcmHeader := stream.generateBcmTransportHeader(stream.msgType)
	buff := [][]byte{bcmHeader.ToBytes()}
	buff = append(buff, dataSlices...)
	stream.sequence = stream.sequence + 1
	if stream.msgType == 0x0 || stream.msgType == 0x1 {
		//fmt.Println("hdr")
		return stream.conn.SendImmediate(buff...)
	} else if stream.msgType == 0x2 {
		//fmt.Println("body")
		return stream.conn.SendWithBuffer(buff...)
	} else if stream.msgType == 0x3 {
		//fmt.Println("end")
		return stream.conn.SendWithBufferFlush(buff...)
	} else {
		return stream.conn.SendImmediate(buff...)
	}
	// return stream.conn.SendImmediate(buff...)
}

func (stream *BcmDCNLightUDPStream) WriteFlush(dataSlices ...[]byte) (int, error) {
	// FIXME: impl
	return stream.Write(dataSlices...)
}

func (stream *BcmDCNLightUDPStream) Read() (data []byte) {
	stream.Lock()
	defer stream.Unlock()
	if len(stream.recvBuffer) == 0 {
		return []byte{}
	} else {
		freeBuffer := func(stream *BcmDCNLightUDPStream) { stream.recvBuffer = stream.recvBuffer[:1] }
		defer freeBuffer(stream)
		return stream.recvBuffer[0]
	}
}

func (stream *BcmDCNLightUDPStream) BlockingRead() (data []byte) {
	// FIXME: impl
	return []byte{}
}

func (stream *BcmDCNLightUDPStream) SetSendTimeout(t int) bool {
	return false
}

func (stream *BcmDCNLightUDPStream) SetReceiveTimeout(t int) bool {
	return false
}

func (stream *BcmDCNLightUDPStream) SetTimeout(t int) bool {
	r := stream.SetSendTimeout(t) && stream.SetReceiveTimeout(t)
	return r
}
