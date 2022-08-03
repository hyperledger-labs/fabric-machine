/*
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fmprotocol

import (
	"io/ioutil"
)

type CertificateInfo struct {
	id            int
	name          string
	ca            []byte
	identity_data []byte
}

var CertificateCache map[string]CertificateInfo
var CertificateIdCache map[int]CertificateInfo

func generateId(userId int, orgId int, role int) (id int) {
	id = role + (orgId << 4) + (userId << 8)
	return id
}

func generateOrgId(data int) (id uint16) {
	id = (uint16)(data & 0x0FF)
	return id
}

func initCertificateCache() {
	CertificateCache = make(map[string]CertificateInfo)
	CertificateIdCache = make(map[int]CertificateInfo)
}

// serializeCertificate generates serialized identity data based on certificate name and certificate data
func serializeCertificate(name string, ca []byte) (id_data []byte) {
	id_data = make([]byte, 0)
	id_data = append(id_data, uint8(0x0a))
	id_data = append(id_data, uint8(len(name)))
	id_data = append(id_data, []byte(name)...)

	id_data = append(id_data, uint8(0x12))
	id_data = append(id_data, uint8((len(ca)&0x7F)|0x80))
	id_data = append(id_data, uint8(len(ca)>>7))
	id_data = append(id_data, ca...)
	return id_data
}

// getCertificateId gets cache ID number of a certificate
func getCertificateId(identity_data []byte) (id int) {
	k := string(identity_data)
	v, prs := CertificateCache[k]
	if prs == false {
		return -1
	} else {
		return v.id
	}
}

// getCertificateById gets a cached certificate based on it's ID number
func getCertificateById(id int) (ca []byte) {
	v, prs := CertificateIdCache[id]
	if prs == false {
		return []byte{}
	} else {
		return v.identity_data
	}
}

// insertCertificate install new certificate to certificate cache
func insertCertificate(id int, name string, ca []byte) (res int) {
	_, prs := CertificateIdCache[id]
	if prs == true {
		return -1 // id existed
	}

	sk := serializeCertificate(name, ca)
	k := getCertificateId(sk)
	if k > 0 {
		return -1 // ca existed
	}

	CertificateCache[string(sk)] = CertificateInfo{id, name, ca, sk}
	CertificateIdCache[id] = CertificateInfo{id, name, ca, sk}
	return 0
}

// removeCertificateFromCache deletes a cached certificate
func removeCertificateFromCache(id int) {
	k := string(CertificateIdCache[id].identity_data)
	delete(CertificateIdCache, id)
	delete(CertificateCache, k)
}

// getCertificateIdWithRuntimeUpdate finds or creates a cache entry based on certificate data and return the cache ID
func getCertificateIdWithRuntimeUpdate(ca []byte) (id int) {
	k := string(ca)
	v, prs := CertificateCache[k]

	if prs == false {
		insertCertificate(len(CertificateCache), "test-"+string(len(CertificateCache)), ca)
		v, prs = CertificateCache[k]
		logger.Warningf("Certificate not found, insert runtime: id=%d", v.id)
	}
	return v.id
}

// installCertificateFile add a certificate file to certificate cache
func installCertificateFile(path string, id int, name string) (ret int) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		logger.Errorf("read file error")
		return -1
	}
	logger.Infof("certificate read from %s done. size = %d", path, len(data))

	ret = insertCertificate(id, name, data)
	return ret
}

// updateRemoteCertificateCache updates the remote peer with latest cache data
func updateRemoteCertificateCache(addr string) {
	for _, v := range CertificateCache {
		sendCertificateCacheUpdate(addr, v.id, v.name, v.ca)
	}
}
