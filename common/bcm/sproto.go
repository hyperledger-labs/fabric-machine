/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package bcm

// getVarint reads a varint value from data position, return the value and next data position
func getVarint(data []byte, pos int) (val int, off int) {
	val = 0
	off = 0
	for data[pos]&0x80 == 0x80 {
		val += int(data[pos]&0x7F) << uint(7*off)
		pos++
		off++
	}
	val += int(data[pos]) << uint(7*off)
	off++
	return val, off
}

// protoGetFieldLength reads field ID, field length and field data position
func protoGetFieldLength(data []byte, pos int) (field int, length int, newPos int) {
	field, off := getVarint(data, pos)
	field_type := field & 0x7
	field = field >> 3
	pos += off

	length, off = getVarint(data, pos)
	if field_type != 0 {
		pos += off
	} else {
		length = off
	}

	return field, length, pos
}

// protoSearchFieldPos finds first fields data position and length from the starting point
func protoSearchFieldPos(data []byte, start int, size int, target_field int) (pos int, length int) {
	pos = start
	end := start + size
	field := 0
	for pos < end {
		field, length, pos = protoGetFieldLength(data, pos)
		if field == target_field {
			return pos, length
		}
		pos += length
	}
	return -1, 0
}

// protoGetFieldPos search field IDs in path recursively, return the last field data position and length
func protoGetFieldPos(data []byte, start int, size int, path []int) (pos int, length int) {
	pos = start
	length = size
	for i := 0; i < len(path); i++ {
		pos, length = protoSearchFieldPos(data, pos, length, path[i])
		if pos == -1 {
			return -1, 0
		}
	}
	return pos, length
}
