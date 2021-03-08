/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// config.go implements utilities to read Fabric machine's configuration data.
package fabricmachine

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

type FabricMachineConfig struct {
	configFile string
	config     *viper.Viper
}

var fmConfig FabricMachineConfig

func InitConfig() error {
	// The Fabric peer codebase uses a global instance of viper to read configuration data, so we
	// use the same to get the Fabric machine's config file.
	configFile := viper.GetString("peer.hw.config.file")
	if len(configFile) == 0 {
		return nil
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
	fmConfig.config = v
	logger.Info("Initialized Fabric machine configuration.")
	return nil
}

func IsEnabled() bool {
	return fmConfig.config != nil
}

func GetPcieResourceFile() string {
	return fmConfig.config.GetString("PcieResourceFile")
}
