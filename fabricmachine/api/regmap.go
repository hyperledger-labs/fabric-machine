/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// regmap.go implements Fabric machine's register map and providies utilities to read/write the
// registers.
package fmapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/hyperledger/fabric/fabricmachine/pcieutil"
)

const (
	kClockPeriod   = time.Duration(4 * time.Nanosecond)
	kBlockMaxTxs   = 256
	kResSize       = 64 + 16 + 1 + kBlockMaxTxs + 64
	kAxilDataWidth = 32
	kNumResRegs    = ((kResSize - 1) / kAxilDataWidth) + 1

	kUlRstRegAddr        = uint32(0x14) // User logic reset register.
	kShellVersionRegAddr = uint32(0x0)
	kFmVersionRegAddr    = uint32(0x20000)
	kResRegsAddr         = uint32(0x10000)

	kUlRstVal = uint32(0xFFFFFFFF)
)

type RegMap struct {
	pcie         *pcieutil.PCIeMemMap
	shellVersion uint32
	fmVersion    uint32
	resRegs      [kNumResRegs]uint32
}

func NewRegMap(pcieResourceFile string) (*RegMap, error) {
	pcie, err := pcieutil.NewPCIeMemMap(pcieResourceFile)
	if err != nil {
		return nil, err
	}
	return &RegMap{pcie: pcie}, nil
}

func (regmap *RegMap) resetSystem() error {
	return regmap.pcie.WriteAt(kUlRstRegAddr, kUlRstVal)
}

func (regmap *RegMap) readSysVersion() error {
	var err error
	if regmap.shellVersion, err = regmap.pcie.ReadAt(kShellVersionRegAddr); err != nil {
		return err
	}
	if regmap.fmVersion, err = regmap.pcie.ReadAt(kFmVersionRegAddr); err != nil {
		return err
	}
	return nil
}

// readResRegs reads the block data related registers.
func (regmap *RegMap) readResRegs(start, end int) error {
	var err error

	// Fabric machine's registers have an internal mechanism to write new values only when all
	// the registers have been read by the software. We must always read from the most significant
	// register to the least significant one to avoid reading partial old and new data.
	for i := end - 1; i >= start; i-- {
		if regmap.resRegs[i], err = regmap.pcie.ReadAt(kResRegsAddr + 4*uint32(i)); err != nil {
			return err
		}
	}
	return nil
}

// getResRegsAsString returns the block data related registers formatted as a string.
// It must be called after readResRegs() so that the internal data structure has the correct data.
func (regmap *RegMap) getResRegsAsString() string {
	var str strings.Builder

	for i := kNumResRegs - 1; i >= 0; i-- {
		str.WriteString(fmt.Sprintf("%#08X ", regmap.resRegs[i]))
	}

	return str.String()
}

// getBlockNum returns the block number by decoding the register values.
// It must be called after readResRegs() so that the internal data structure has the correct data.
func (regmap *RegMap) getBlockNum() uint64 {
	var num uint64
	num = uint64(regmap.resRegs[kNumResRegs-1])
	num = (num << kAxilDataWidth) | uint64(regmap.resRegs[kNumResRegs-2])
	num = (num << (kAxilDataWidth - 17)) | uint64(regmap.resRegs[kNumResRegs-3]>>17)
	return num
}

// getBlockNum returns the block number by decoding the register values.
// It must be called after readResRegs() so that the internal data structure has the correct data.
func (regmap *RegMap) getBlockNumTxs() uint32 {
	return (regmap.resRegs[kNumResRegs-3] & 0x0001FFFE) >> 1
}

// isBlockValid returns true if the block is valid (by decoding the register values).
// It must be called after readResRegs() so that the internal data structure has the correct data.
func (regmap *RegMap) isBlockValid() bool {
	return (regmap.resRegs[kNumResRegs-3] & 0x1) == 1
}

// getBlockTxsVldFlags returns the validation flags of txs by decoding the register values.
// It must be called after readResRegs() so that the internal data structure has the correct data.
func (regmap *RegMap) getBlockTxsVldFlags() [kBlockMaxTxs]uint8 {
	var vldFlags [kBlockMaxTxs]uint8

	for i := 0; i < (kBlockMaxTxs / kAxilDataWidth); i++ {
		r := regmap.resRegs[2+i]
		for j := 0; j < kAxilDataWidth; j++ {
			if (r & 0x1) == 1 {
				vldFlags[i*kAxilDataWidth+j] = uint8(1)
			} else {
				vldFlags[i*kAxilDataWidth+j] = uint8(0)
			}
			r >>= 1
		}
	}
	return vldFlags
}

// getBlockNum returns the block latency by decoding the register values.
// It must be called after readResRegs() so that the internal data structure has the correct data.
func (regmap *RegMap) getBlockLatency() time.Duration {
	var latency uint64
	latency = uint64(regmap.resRegs[1])
	latency = ((latency << kAxilDataWidth) | uint64(regmap.resRegs[0]))
	return (time.Duration(latency) * kClockPeriod)
}
