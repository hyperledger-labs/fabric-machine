/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package bcm

// Blockchain machine protocol message stream
type BcmSendStream interface {
	SendStreamID() int
	Write(dataSlices ...[]byte) (int, error)
	WriteFlush(dataSlices ...[]byte) (int, error)
	SetSendTimeout(t int) bool // FIXME:
}

type BcmReceiveStream interface {
	ReceiveStreamID() int
	Read() (data []byte)
	BlockingRead() (data []byte)
	SetReceiveTimeout(t int) bool // FIXME:
}

type BcmStream interface {
	StreamID() int
	BcmSendStream
	BcmReceiveStream
	SetTimeout(t int) bool
}

// Blockchain machine protocol peer to peer connection
type BcmConnection interface {
	OpenStream(msgType byte) BcmStream
	CloseStream(msgType byte)
	LocalAddr() string
	RemoteAddr() string
	ConnectionState() int
	StartListen()
}
