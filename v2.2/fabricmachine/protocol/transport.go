/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package fmprotocol

import (
	"bytes"
	"encoding/binary"
	"net"
)

// BcmSession keeps blockchain machine protocol session information
type BcmSession struct {
	addr     string
	sequence uint16
	info     uint32
	udpConn  *net.UDPConn
}

// BcmSessionMap records IP to blockchain machine protocol session relation
var BcmSessionMap map[string]BcmSession

type BcmTransportHeader struct {
	sequence        uint16
	ctrl_type       uint8
	annotation_size uint8
}

// transportHeaderToBytes formats blockchain machine protocol header to binary
func transportHeaderToBytes(hdr BcmTransportHeader) (data []byte) {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, hdr)
	data = buf.Bytes()
	return data
}

// bcmSessionFindOrCreate searches session map for IP, creates a new session if not existed
func bcmSessionFindOrCreate(addr string) (session BcmSession) {
	if BcmSessionMap == nil {
		BcmSessionMap = make(map[string]BcmSession)
	}
	if BcmSessionMap[addr].addr != addr {
		udpAddr, _ := net.ResolveUDPAddr("udp4", addr)
		udpConn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			return
		}
		BcmSessionMap[addr] = BcmSession{addr, 0, 0, udpConn}
	}
	return BcmSessionMap[addr]
}

// bcmSessionSend forms a packet from blockchain machine protocol message and send out via protocol session
func bcmSessionSend(session BcmSession, msgType byte, annotation_data []byte, annotation_num int, payload []byte) {
	control := 0

	bcmHeader := BcmTransportHeader{session.sequence, uint8(byte(control<<4) | msgType), uint8(annotation_num)}
	var buff []byte
	buff = append(buff, transportHeaderToBytes(bcmHeader)...)
	buff = append(buff, annotation_data...)
	buff = append(buff, payload...)

	length, err := session.udpConn.Write(buff)
	if err != nil {
		return
	}

	if length > 9000 {
		logger.Errorf("error: UDP too large")
	} else {
		logger.Debugf("send ok: ", len(annotation_data), len(payload), length)
	}
}

// bcmSend sends a blockchain machine protocol message to destination IP address
func bcmSend(addr string, msgType byte, annotation_data []byte, annotation_num int, payload []byte) {
	session := bcmSessionFindOrCreate(addr)
	bcmSessionSend(session, msgType, annotation_data, annotation_num, payload)
}
