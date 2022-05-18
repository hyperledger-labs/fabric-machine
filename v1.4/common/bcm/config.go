/*
Copyright Xilinx Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package bcm

import (
	"github.com/spf13/viper"
)

// Orgnization in config file
type Organization struct {
	Name  string
	Certs []struct {
		Role string
		Cert string
	}
}

// loadIdentityTable read certificates from config file and write to certificate table
func loadIdentityTable(roles []string, organizations []Organization) (errno int) {
	errno = 0
	role_to_id := make(map[string]int)
	for role_id, role := range roles {
		role_to_id[role] = role_id
	}

	for org_id, organization := range organizations {
		user_id_record := [4]int{0, 0, 0, 0}
		for _, cer := range organization.Certs {
			role_id := role_to_id[cer.Role]
			org_name := organization.Name
			user_id := user_id_record[role_id]
			cer_path := cer.Cert
			installCertificateFile(cer_path, generateId(user_id, org_id, role_id), org_name)
			user_id_record[role_id]++
		}
	}
	return errno
}

// readConfig reads config file and call related functions
func readConfig(path string) int {
	viper.SetConfigName("fabric_machine") // name of config file (without extension)
	viper.SetConfigType("yaml")           // REQUIRED if the config file does not have the extension in the name
	// FIXME: change this path with correct config path
	viper.AddConfigPath(path)   // path to look for the config file in
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		logger.Errorf("Fatal error config file: %s %s\n", path, err)
		return -1
	}

	roles := viper.GetStringSlice("Roles")
	organizations := make([]Organization, 0)
	viper.UnmarshalKey("Organizations", &organizations)

	loadIdentityTable(roles, organizations)
	return 0
}
