/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package pcieutil contains utilities to read from and write to registers of a PCIe device.
package pcieutil

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type PCIeMemMap struct {
	resourceFile string
	fd           *os.File
	memMap       []byte
}

// NewPCIeMemMap creates a memory map for the PCIe device represented by the provided resource file.
func NewPCIeMemMap(resourceFile string) (*PCIeMemMap, error) {
	resourceFileInfo, err := os.Stat(resourceFile)
	if err != nil {
		return nil, err
	}

	fd, err := os.OpenFile(resourceFile, os.O_RDWR|os.O_SYNC, 0666)
	if err != nil {
		return nil, err
	}

	memMap, err := syscall.Mmap(int(fd.Fd()), 0, int(resourceFileInfo.Size()),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("Could not map memory for %v: %v", resourceFile, err.Error())
	}

	return &PCIeMemMap{resourceFile, fd, memMap}, nil
}

// Close unmaps the memory for the PCIe device, and cleans up.
func (pcie *PCIeMemMap) Close() error {
	if pcie.memMap == nil {
		return nil
	}

	if err := syscall.Munmap(pcie.memMap); err != nil {
		return fmt.Errorf("Could not unmap memory for %v: %v", pcie.resourceFile, err.Error())
	}
	pcie.memMap = nil
	pcie.fd.Close()

	return nil
}

// ReadAt reads from the PCIe device register at the provided offset (should be word-aligned).
func (pcie *PCIeMemMap) ReadAt(offset uint32) (uint32, error) {
	if pcie.memMap == nil {
		return 0, fmt.Errorf("Memory map doesn't exist for %v", pcie.resourceFile)
	}

	addr := (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(&pcie.memMap[0])) + uintptr(offset)))
	val := *addr
	return val, nil
}

// WriteAt writes to the PCIe device register at the provided offset (should be word-aligned).
func (pcie *PCIeMemMap) WriteAt(offset, data uint32) error {
	if pcie.memMap == nil {
		return fmt.Errorf("Memory map doesn't exist for %v", pcie.resourceFile)
	}

	*(*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(&pcie.memMap[0])) + uintptr(offset))) = data
	return nil
}
