/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package bcm

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

// BcmTransportConn keeps end to end transport connection
type BcmDCNLightUDPConn struct {
	sync.RWMutex
	localAddr    string
	remoteAddr   string
	udpConn      *net.UDPConn
	packetConn   *ipv4.PacketConn
	listenerConn net.PacketConn
	// send
	wms *[]ipv4.Message
	cm  ipv4.ControlMessage
	// receive
	rms    *[]ipv4.Message
	ctx    context.Context
	cancel context.CancelFunc
	// stream
	streams map[byte]*BcmDCNLightUDPStream
	status  int
	strmID  int
}

func newDCNLightUDPConn(localAddr string, remoteAddr string) *BcmDCNLightUDPConn {
	udpAddr, _ := net.ResolveUDPAddr("udp4", remoteAddr)
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil
	}
	packetConn := ipv4.NewPacketConn(udpConn)
	wms := []ipv4.Message{}
	rms := []ipv4.Message{}
	cm := ipv4.ControlMessage{
		//Src: net.IPv4(127, 0, 0, 1),
		Src: net.ParseIP(localAddr),
	}

	listenerConn, err := net.ListenPacket("udp", localAddr) // FIXME:
	if err != nil {
		fmt.Println("Error Listening ", err.Error())
		panic(err)
	}

	ss := make(map[byte]*BcmDCNLightUDPStream)
	ctx, cancel := context.WithCancel(context.Background())
	return &BcmDCNLightUDPConn{
		localAddr:    localAddr,
		remoteAddr:   remoteAddr,
		udpConn:      udpConn,
		packetConn:   packetConn,
		listenerConn: listenerConn,
		wms:          &wms,
		cm:           cm,
		rms:          &rms,
		ctx:          ctx,
		cancel:       cancel,
		streams:      ss,
		status:       0,
		strmID:       0,
	}
}

func (conn *BcmDCNLightUDPConn) LocalAddr() string {
	return conn.localAddr
}

func (conn *BcmDCNLightUDPConn) RemoteAddr() string {
	return conn.remoteAddr
}

func (conn *BcmDCNLightUDPConn) ConnectionState() int {
	return conn.status
}

func (conn *BcmDCNLightUDPConn) OpenStream(msgType byte) BcmStream {
	conn.Lock()
	defer conn.Unlock()
	conn.strmID = conn.strmID + 1
	stream := newDCNLightUDPStream(conn.strmID, conn, msgType)
	conn.streams[msgType] = stream
	return stream
}
func (conn *BcmDCNLightUDPConn) CloseStream(msgType byte) {
	conn.Lock()
	defer conn.Unlock()
	delete(conn.streams, msgType)
}

func (conn *BcmDCNLightUDPConn) SendImmediate(dataSlices ...[]byte) (int, error) {
	var datagram []byte
	for i := 0; i < len(dataSlices); i++ {
		datagram = append(datagram, dataSlices[i]...)
	}

	length, err := conn.packetConn.WriteTo(datagram, &conn.cm, conn.udpConn.RemoteAddr())
	return length, err
}

func (conn *BcmDCNLightUDPConn) SendWithBuffer(dataSlices ...[]byte) (int, error) {
	conn.Lock()
	defer conn.Unlock()
	//m := ipv4.Message{Buffers: [][]byte{transportHeaderToBytes(bcmHeader)}}
	//m.Buffers = append(m.Buffers, dataSlices...)
	m := ipv4.Message{Buffers: dataSlices}
	*(conn.wms) = append(*(conn.wms), m)
	// FIXME: calculate length
	return 0, nil
}

func (conn *BcmDCNLightUDPConn) SendWithBufferFlush(dataSlices ...[]byte) (int, error) {
	conn.SendWithBuffer(dataSlices...) // FIXME: not efficiency
	conn.Lock()
	defer conn.Unlock()
	length, err := conn.packetConn.WriteBatch(*conn.wms, 0)
	*conn.wms = (*conn.wms)[:0]
	return length, err
}

func (conn *BcmDCNLightUDPConn) StartListen() {
	conn.Lock()
	defer conn.Unlock()
	go conn.BcmListener()
}

func (conn *BcmDCNLightUDPConn) BcmListener() {
	buffer := make([]byte, 2048)
	defer conn.listenerConn.Close()
	for {
		select {
		case <-conn.ctx.Done():
			fmt.Println("Bcm Protocol listener server ...")
			return
		default:
			conn.listenerConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			size, remoteAddr, _ := conn.listenerConn.ReadFrom(buffer[0:]) //TODO: check read funcs
			//if err != nil {		//NOTE: timeout warnings are hided
			//fmt.Println("Error data reading", err.Error())
			//}
			if size > 4 && remoteAddr != nil {
				conn.onRecv(buffer[:size]) // copy once
			}
		}
	}
}

func (conn *BcmDCNLightUDPConn) onRecv(message []byte) {
	conn.Lock()
	msgType := message[2] & 0xF
	stream, existed := conn.streams[msgType]
	conn.Unlock()
	if existed == true {
		stream.Lock()
		m := make([]byte, len(message))
		copy(m, message)
		stream.recvBuffer = append(stream.recvBuffer, m)
		stream.Unlock()
	}
}
