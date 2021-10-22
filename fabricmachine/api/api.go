/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package fmapi provides the interface to interact with the Fabric machine (hardware
// accelerators implemented on an FPGA board, accessed through PCIe).
package fmapi

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/protos/peer"
)

var logger = flogging.MustGetLogger("fabricmachine")

type FabricMachine struct {
	regmap *RegMap
}

type BlockData struct {
	Num         uint64
	NumTxs      uint32
	Valid       bool
	TxsVldFlags util.TxValidationFlags
	Latency     time.Duration
}

// NewFabricMachine returns a Fabric machine instance.
// It resets the Fabric machine to ensure a consistent initial state.
func NewFabricMachine(pcieResourceFile string) (*FabricMachine, error) {
	logger.Infof("Initializing Fabric machine with %s ...", pcieResourceFile)
	regmap, err := NewRegMap(pcieResourceFile)
	if err != nil {
		return nil, err
	}

	regmap.resetSystem()
	logger.Info("Fabric machine has been reset.")
	regmap.readSysVersion()
	logger.Infof("OpenNIC build version: 0x%x Fabric machine build version: 0x%x\n", regmap.shellVersion, regmap.fmVersion)

	return &FabricMachine{regmap}, nil
}

func (fm *FabricMachine) Close() error {
	return fm.regmap.pcie.Close()
}

// getBlockTxsVldFlags returns the validation flags of txs as the type used by Fabric software.
func (fm *FabricMachine) getBlockTxsVldFlags() util.TxValidationFlags {
	numTxs := int(fm.regmap.getBlockNumTxs())
	fmVldFlags := fm.regmap.getBlockTxsVldFlags()
	vldFlags := util.NewTxValidationFlagsSetValue(numTxs, peer.TxValidationCode_VALID)

	for i := 0; i < numTxs; i++ {
		if fmVldFlags[i] == 0 {
			// For now, we use mvcc as the reason for every invalid transction. In future, the
			// hardware should be updated to also report the reason for invalid transactions.
			// vldFlags.SetFlag(i, peer.TxValidationCode_INVALID_OTHER_REASON)
			vldFlags.SetFlag(i, peer.TxValidationCode_MVCC_READ_CONFLICT)
		}
	}
	return vldFlags
}

// GetBlockData returns block data after reading from the Fabric machine.
// This function will only return when the block is available, unless numTries > 0 in which case it
// will return after trying for numTries.
// Do not read a block again after it has been successfully read, because that messes up the
// synchronization between the hardware and software. If you do that, data returned for subsequent
// blocks may be corrupted.
func (fm *FabricMachine) GetBlockData(blockNum uint64, numTries int) (*BlockData, error) {
	rm := fm.regmap
	iters := 0

	logger.Infof("Reading block %d ...", blockNum)
	for {
		// Read the most significant 3 registers to see whether the data for the expected
		// block is available or not.
		if err := rm.readResRegs(kNumResRegs-3, kNumResRegs); err != nil {
			return nil, err
		}
		iters++
		bn := rm.getBlockNum()
		// logger.Infof("Got block %d", bn)

		if bn != blockNum || (blockNum == 0 && !rm.isBlockValid()) { // Block0 must always be valid.
			if numTries == 0 || iters < numTries {
				continue
			}

			return &BlockData{Num: bn}, fmt.Errorf("Expected block %d but Fabric machine has block %d", blockNum, bn)
		}

		// Now, we have the expected block. Let's read all of its data.
		if err := rm.readResRegs(0, kNumResRegs); err != nil {
			return nil, err
		}
		logger.Infof("Got block %d ...", bn)

		return &BlockData{
			Num:         rm.getBlockNum(),
			NumTxs:      rm.getBlockNumTxs(),
			Valid:       rm.isBlockValid(),
			TxsVldFlags: fm.getBlockTxsVldFlags(),
			Latency:     rm.getBlockLatency(),
		}, nil
	}
}
