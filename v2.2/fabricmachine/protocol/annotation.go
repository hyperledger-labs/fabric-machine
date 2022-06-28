/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fmprotocol

import (
	"bytes"
	"encoding/binary"
)

const MAX_ENDORSER_NUM int = 8

type AnnotationPointer struct {
	dataType uint8
	offset   uint16
	length   uint16
}

type AnnotationLocator struct {
	dataType    uint8
	offset      uint16
	description uint16
}

type Annotation struct {
	dataType uint8
	offset   uint16
	desc     uint16 // desc/len
	//data uint32
}

// annotation mask
const ANNOTATION_TYPE_MASK byte = 0x80
const ANNOTATION_DATA_TYPE_MASK byte = 0x7F

// annotation type
const ANNOTATION_TYPE_POINTER byte = 0x00
const ANNOTATION_TYPE_LOCATOR byte = 0x80

// annotations for block header message
const ANNOTATION_DATA_TYPE_BLOCKHEADER byte = 0x10
const ANNOTATION_DATA_TYPE_TRANSACTION byte = 0x11
const ANNOTATION_DATA_TYPE_BLOCKMETADATA byte = 0x12
const ANNOTATION_DATA_TYPE_BLOCK_ID byte = 0x13

// annotations for transaction
const ANNOTATION_DATA_TYPE_TX_START byte = 0x20
const ANNOTATION_DATA_TYPE_CHANNEL_NAME byte = 0x21
const ANNOTATION_DATA_TYPE_TX_ID byte = 0x22
const ANNOTATION_DATA_TYPE_CHAINCODE_NAME byte = 0x23
const ANNOTATION_DATA_TYPE_CREATER_CA byte = 0x24
const ANNOTATION_DATA_TYPE_TX_ACTION byte = 0x25
const ANNOTATION_DATA_TYPE_TX_CA byte = 0x26
const ANNOTATION_DATA_TYPE_CONTRACT_NAME byte = 0x27
const ANNOTATION_DATA_TYPE_CONTRACT_INPUT byte = 0x28
const ANNOTATION_DATA_TYPE_ENDORSER_ACTION byte = 0x29
const ANNOTATION_DATA_TYPE_RW_SET byte = 0x2a
const ANNOTATION_DATA_TYPE_ENDORSER byte = 0x2b
const ANNOTATION_DATA_TYPE_ENDORSER_CA byte = 0x2c
const ANNOTATION_DATA_TYPE_ENDORSER_SIG byte = 0x2d
const ANNOTATION_DATA_TYPE_TX_SIG byte = 0x2e

// annotations for block metata
const ANNOTATION_DATA_TYPE_ORDERER_CA byte = 0x40
const ANNOTATION_DATA_TYPE_ORDERER_SIG byte = 0x41

// annotations for cache update message
const ANNOTATION_DATA_TYPE_CACHE_DATA byte = 0x01
const ANNOTATION_DATA_TYPE_CACHE_NAME byte = 0x02
const ANNOTATION_DATA_TYPE_CACHE_CA byte = 0x03

type AnnotationInfo struct {
	annotationType byte
	path           []int
}

var blockHeaderAnnotationInfoList []AnnotationInfo = []AnnotationInfo{
	// block header start position
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_BLOCKHEADER, []int{}},
	// transaction numbers and position
	{ANNOTATION_TYPE_LOCATOR | ANNOTATION_DATA_TYPE_TRANSACTION, []int{}},
	// block metadata position
	{ANNOTATION_TYPE_LOCATOR | ANNOTATION_DATA_TYPE_BLOCKMETADATA, []int{}},
	// block id
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_BLOCK_ID, []int{}},
}

var blockTransactionAnnotationInfoList []AnnotationInfo = []AnnotationInfo{
	// transaction start
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_TX_START, []int{1, 2}},
	// transaction id
	// {ANNOTATION_TYPE_POINTER|ANNOTATION_DATA_TYPE_TX_ID, []int{1, 1, 1, 1, 5}}, //not required by hw accelerator
	// Chaincode name
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_CHAINCODE_NAME, []int{1, 1, 1, 1, 7, 2, 2}},
	// Creater CA/identity
	{ANNOTATION_TYPE_LOCATOR | ANNOTATION_DATA_TYPE_CREATER_CA, []int{1, 1, 1, 2, 1}},
	// TX action
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_TX_ACTION, []int{1, 1, 2, 1}},
	// TX CA/identity
	{ANNOTATION_TYPE_LOCATOR | ANNOTATION_DATA_TYPE_TX_CA, []int{1, 1, 2, 1, 1, 1}},
	// TODO: smart contract name
	// TODO: smart contract input
	// Endorser action
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_ENDORSER_ACTION, []int{1, 1, 2, 1, 2, 1}},
	// Read/Write set
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_RW_SET, []int{1, 1, 2, 1, 2, 2, 1, 2, 1}},
	// Endorser
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_ENDORSER, []int{1, 1, 2, 1, 2, 2, 2}},
	// Endorser CA/identity list
	{ANNOTATION_TYPE_LOCATOR | ANNOTATION_DATA_TYPE_ENDORSER_CA, []int{1, 1, 2, 1, 2, 2, 2}},
	// Transaction Signature
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_TX_SIG, []int{1, 2}},
}

var blockMetadataAnnotationInfoList []AnnotationInfo = []AnnotationInfo{
	// Orderer CA/Identity
	{ANNOTATION_TYPE_LOCATOR | ANNOTATION_DATA_TYPE_ORDERER_CA, []int{3, 1, 2, 1, 1}},
	// block signature
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_ORDERER_SIG, []int{3, 1, 2, 2}},
}

var blockCacheUpdateAnnotationInfoList []AnnotationInfo = []AnnotationInfo{
	// Cache ID number for the CA/Identity data
	{ANNOTATION_TYPE_LOCATOR | ANNOTATION_DATA_TYPE_CACHE_DATA, []int{}},
	// Identity name
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_CACHE_NAME, []int{}},
	// Certificate data
	{ANNOTATION_TYPE_POINTER | ANNOTATION_DATA_TYPE_CACHE_CA, []int{}},
}

var BlockHeaderAnnotationNumber int = len(blockHeaderAnnotationInfoList)
var BlockTransactionAnnotationNumber int = len(blockTransactionAnnotationInfoList) - 1 + MAX_ENDORSER_NUM
var BlockMetadataAnnotationNumber int = len(blockMetadataAnnotationInfoList)
var CacheUpdateAnnotationNumber int = len(blockCacheUpdateAnnotationInfoList)

func makeAnnotation(dataType uint8, offset uint16, desc uint16) (annotation Annotation) {
	annotation = Annotation{dataType, offset, desc}
	return annotation
}

func getAnnotationDesc(annotation Annotation) (desc uint16) {
	desc = annotation.desc
	return desc
}

func getAnnotationOffset(annotation Annotation) (offset uint16) {
	offset = annotation.offset
	return offset
}

func getAnnotationDataType(annotation Annotation) (dataType uint8) {
	dataType = annotation.dataType
	return dataType
}

func setAnnotationDesc(annotation *Annotation, desc uint16) {
	annotation.desc = desc
}

func setAnnotationOffset(annotation *Annotation, offset uint16) {
	annotation.offset = offset
}

// annotationListToBytes serializes annotations to big endian
func annotationListToBytes(annotation []Annotation) (data []byte) {
	buf := &bytes.Buffer{}
	for i := 0; i < len(annotation); i++ {
		binary.Write(buf, binary.BigEndian, annotation[i])
	}
	data = buf.Bytes()
	return data
}

// generateBlockAnnotation generates annotation list for block header message
func generateBlockAnnotation(hdr_pos int, hdr_len int, transaction_len_list []int, data []byte) (ret []Annotation) {
	ret = make([]Annotation, BlockHeaderAnnotationNumber)

	for i, annotationInfo := range blockHeaderAnnotationInfoList {
		switch annotationInfo.annotationType & ANNOTATION_DATA_TYPE_MASK {
		case ANNOTATION_DATA_TYPE_BLOCKHEADER:
			ret[i] = makeAnnotation(annotationInfo.annotationType, uint16(hdr_pos), uint16(hdr_len))
		case ANNOTATION_DATA_TYPE_TRANSACTION:
			ret[i] = makeAnnotation(annotationInfo.annotationType, uint16(len(transaction_len_list)), 0)
		case ANNOTATION_DATA_TYPE_BLOCKMETADATA:
			ret[i] = makeAnnotation(annotationInfo.annotationType, 1, 0)
		case ANNOTATION_DATA_TYPE_BLOCK_ID:
			pos := 0
			length := 0
			// block ID : varint
			_, length, pos = protoGetFieldLength(data, pos)
			pos, length = protoGetFieldPos(data, 0, length, []int{1})
			length = 1
			pos = pos + 1
			pos_temp := pos
			for pos_temp < 64 {
				if (data[pos_temp] & 0x80) != 0x80 {
					break
				}
				pos_temp = pos_temp + 1
				length = length + 1
			}
			ret[i] = makeAnnotation(annotationInfo.annotationType, uint16(pos), uint16(length))
		}
	}
	return ret
}

// generateTransactionAnnotation generates annotation list for transaction
func generateTransactionAnnotation(transaction_pos int, transaction_len int, data []byte) (ret []Annotation) {
	ret = make([]Annotation, BlockTransactionAnnotationNumber)
	pos := transaction_pos
	length := 0
	endorser_end := 0

	i := 0

	for _, annotationInfo := range blockTransactionAnnotationInfoList {
		switch annotationInfo.annotationType & ANNOTATION_DATA_TYPE_MASK {
		case ANNOTATION_DATA_TYPE_ENDORSER:
			pos, length = protoGetFieldPos(data, transaction_pos, transaction_len, annotationInfo.path[:len(annotationInfo.path)-1])
			endorser_end = pos + length

			pos, length = protoGetFieldPos(data, transaction_pos, transaction_len, annotationInfo.path)
			ret[i] = makeAnnotation(annotationInfo.annotationType, uint16(pos-transaction_pos-3), uint16(length)) // move annotation back to endorser start point (1B type, 2B length)
			i++
		case ANNOTATION_DATA_TYPE_ENDORSER_CA:
			j := 0
			pos, length = protoGetFieldPos(data, transaction_pos, transaction_len, annotationInfo.path)
			for j = 0; j < MAX_ENDORSER_NUM; j++ {
				_, length, pos = protoGetFieldLength(data, pos)
				ret[i] = makeAnnotation(annotationInfo.annotationType, uint16(pos-transaction_pos), uint16(length))
				i++
				// skip signature
				pos += length
				_, length, pos = protoGetFieldLength(data, pos)

				pos += length
				if pos >= endorser_end {
					break
				}
				_, length, pos = protoGetFieldLength(data, pos)
			}
			j = j + 1
			setAnnotationDesc(&(ret[i-j-1]), uint16(j))
			for ; j < MAX_ENDORSER_NUM; j++ {
				ret[i] = makeAnnotation(0, 0, 0)
				i++
			}
		default:
			pos, length = protoGetFieldPos(data, transaction_pos, transaction_len, annotationInfo.path)
			ret[i] = makeAnnotation(annotationInfo.annotationType, uint16(pos-transaction_pos), uint16(length))
			i++
		}
	}

	return ret
}

// generateBlockMetaAnnotation generates annotation list for block metadata
func generateBlockMetaAnnotation(metadata_pos int, metadata_len int, data []byte) (ret []Annotation) {
	pos := metadata_pos
	length := 0
	ret = make([]Annotation, BlockMetadataAnnotationNumber)

	for i, annotationInfo := range blockMetadataAnnotationInfoList {
		switch annotationInfo.annotationType & ANNOTATION_DATA_TYPE_MASK {
		default:
			pos, length = protoGetFieldPos(data, metadata_pos, metadata_len, annotationInfo.path)
			ret[i] = makeAnnotation(annotationInfo.annotationType, uint16(pos-metadata_pos), uint16(length))
		}
	}

	return ret
}

// adjustDataBasedOnLocator removes cached data based on ID number in locators
func adjustDataBasedOnLocator(data []byte, pos int, length int, annotation []Annotation) (payload []byte) {
	// Keep original position/length
	start := pos
	end := pos + length
	// removed bytes
	removedBytes := 0
	// search Locator
	var ret []byte
	for i := 0; i < len(annotation); i++ {
		atype := getAnnotationDataType(annotation[i])
		if atype == 0 {
			continue
		} else if atype&ANNOTATION_TYPE_MASK == ANNOTATION_TYPE_LOCATOR {
			id := getCertificateId(data[pos+int(getAnnotationOffset(annotation[i])) : pos+int(getAnnotationOffset(annotation[i])+getAnnotationDesc(annotation[i]))])
			end = pos + int(getAnnotationOffset(annotation[i]))
			ret = append(ret, data[start:end]...)
			start = end + int(getAnnotationDesc(annotation[i]))
			end = start

			correctedOffset := int(getAnnotationOffset(annotation[i])) - removedBytes
			setAnnotationOffset(&(annotation[i]), uint16(correctedOffset))
			removedBytes += int(getAnnotationDesc(annotation[i]))
			// FIXME: work around
			// setAnnotationDesc(&(annotation[i]), uint16(id))
			setAnnotationDesc(&(annotation[i]), uint16(id))
		} else {
			correctedOffset := int(getAnnotationOffset(annotation[i])) - removedBytes
			setAnnotationOffset(&(annotation[i]), uint16(correctedOffset))
		}
	}
	end = pos + length
	if start != end {
		ret = append(ret, data[start:end]...)
	}
	return ret
}

// getAnnotationActiveSize returns active annotation size
func getAnnotationActiveSize(annotation []Annotation) (ret int) {
	ret = 0
	for i := 0; i < len(annotation); i++ {
		atype := getAnnotationDataType(annotation[i])
		if atype == 0 {
			ret--
		}
	}
	ret = ret + len(annotation)
	return ret
}

// generateCertificateUpdateAnnotation generates annotation list for certificate cache update message
func generateCertificateUpdateAnnotation(id int, name string, ca []byte) (payload []byte, annotations []Annotation) {
	// 0a len name
	// 12 len ca
	payload = make([]byte, 0)
	payload = append(payload, uint8(0x0a))
	payload = append(payload, uint8(len(name)))
	payload = append(payload, []byte(name)...)

	payload = append(payload, uint8(0x12))
	payload = append(payload, uint8((len(ca)&0x7F)|0x80))
	payload = append(payload, uint8(len(ca)>>7))
	payload = append(payload, ca...)

	annotations = make([]Annotation, CacheUpdateAnnotationNumber)
	for i, annotationInfo := range blockHeaderAnnotationInfoList {
		switch annotationInfo.annotationType & ANNOTATION_DATA_TYPE_MASK {
		case ANNOTATION_DATA_TYPE_CACHE_DATA:
			annotations[i] = makeAnnotation(annotationInfo.annotationType, uint16(id), uint16(len(name)+len(ca)+5))
		case ANNOTATION_DATA_TYPE_CACHE_NAME:
			annotations[i] = makeAnnotation(annotationInfo.annotationType, uint16(2), uint16(len(name)))
		case ANNOTATION_DATA_TYPE_CACHE_CA:
			annotations[i] = makeAnnotation(annotationInfo.annotationType, uint16(5+len(name)), uint16(len(ca)))
		}
	}
	return payload, annotations
}
