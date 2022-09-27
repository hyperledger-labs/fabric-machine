/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// config.go implements utilities to read Fabric machine's configuration data.
package fmapi

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

type FabricMachineConfig struct {
	configFile   string
	configReader *viper.Viper

	pcieResourceFile string
	resetFpgaCard    bool

	address       string
	orderers      []string
	startingBlock uint64

	swStateDbEnabled bool
}

var fmConfig FabricMachineConfig

func readConfig() {
	fmConfig.pcieResourceFile = fmConfig.configReader.GetString("hardware.pcieResourceFile")
	fmConfig.resetFpgaCard = fmConfig.configReader.GetBool("hardware.resetFpgaCard")

	fmConfig.address = fmConfig.configReader.GetString("hardware.protocol.address")
	fmConfig.orderers = fmConfig.configReader.GetStringSlice("hardware.protocol.orderers")
	fmConfig.startingBlock = uint64(fmConfig.configReader.GetInt("hardware.protocol.startingBlock"))

	fmConfig.swStateDbEnabled = fmConfig.configReader.GetBool("hardware.swStateDbEnabled")
}

func InitConfig(fabricConfigReader *viper.Viper) error {
	// The Fabric codebase either uses a global instance of viper (for peer configuration) or a
	// local instance of viper (for orderer configuration), so we use the same to get Fabric machine's
	// config file. Corresponding environment variables are:
	//   Orderers: ORDERER_FABRIC_HW_CONFIG_FILE
	//   Peers: CORE_FABRIC_HW_CONFIG_FILE
	var configFile string
	if fabricConfigReader != nil {
		configFile = fabricConfigReader.GetString("fabric.hw.config.file")
	} else {
		configFile = viper.GetString("fabric.hw.config.file")
	}

	if len(configFile) == 0 {
		logger.Info("Did not find any hardware config file")
		return nil
	} else {
		logger.Infof("Initializing Fabric machine configuration from %s", configFile)
	}

	v := viper.New()
	dir := filepath.Dir(configFile)
	name := filepath.Base(configFile)
	ext := filepath.Ext(configFile)
	name = name[:len(name)-len(ext)] // Remove extension.
	ext = ext[1:]                    // Remove the preceding dot.
	v.SetConfigName(name)
	v.SetConfigType(ext)
	v.AddConfigPath(dir)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("Could not read config file %v: %v", configFile, err.Error())
	}

	fmConfig.configFile = configFile
	fmConfig.configReader = v
	readConfig()
	logger.Info("Initialized Fabric machine configuration.")
	return nil
}

func IsEnabled() bool {
	return fmConfig.configReader != nil
}

func GetPcieResourceFile() string {
	return fmConfig.pcieResourceFile
}

func ResetFpgaCard() bool {
	return fmConfig.resetFpgaCard
}

func GetHardwareAddress() string {
	return fmConfig.address
}

func GetOrderers() []string {
	return fmConfig.orderers
}

func GetStartingBlock() uint64 {
	return fmConfig.startingBlock
}

func IsSwStateDbEnabled() bool {
	return fmConfig.swStateDbEnabled
}
