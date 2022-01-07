/*
 * Copyright (c) 2019 Huawei Technologies Co., Ltd.
 * MeshAccelerating is licensed under the Mulan PSL v2.
 * You can use this software according to the terms and conditions of the Mulan PSL v2.
 * You may obtain a copy of Mulan PSL v2 at:
 *     http://license.coscl.org.cn/MulanPSL2
 * THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
 * PURPOSE.
 * See the Mulan PSL v2 for more details.
 * Author: LemmyHuang
 * Create: 2021-10-09
 */

package option

// #cgo CFLAGS: -I../../bpf/include
// #include "config.h"
import "C"
import (
	"flag"
	"fmt"
)

const (
	ClientModeKube = "kubernetes"
	ClientModeEnvoy = "envoy"
)

var (
	config	DaemonConfig
)

type BpfConfig struct {
	BpfFsPath	string
	Cgroup2Path	string
}
type ClientConfig struct {
	ClientMode		string
	KubeInCluster	bool
}

type DaemonConfig struct {
	BpfConfig
	ClientConfig
	Protocol	map[string]bool
}

func InitializeDaemonConfig() error {
	dc := &config

	dc.Protocol = make(map[string]bool)
	dc.Protocol["IPV4"]  = C.KMESH_ENABLE_IPV4 == C.KMESH_MODULE_ON
	dc.Protocol["IPV6"]  = C.KMESH_ENABLE_IPV6 == C.KMESH_MODULE_ON
	dc.Protocol["TCP"]   = C.KMESH_ENABLE_TCP == C.KMESH_MODULE_ON
	dc.Protocol["UDP"]   = C.KMESH_ENABLE_UDP == C.KMESH_MODULE_ON
	dc.Protocol["HTTP"]  = C.KMESH_ENABLE_HTTP == C.KMESH_MODULE_ON
	dc.Protocol["HTTPS"] = C.KMESH_ENABLE_HTTPS == C.KMESH_MODULE_ON

	flag.StringVar(&dc.BpfConfig.BpfFsPath, "bpfFsPath", "/sys/fs/bpf/", "bpf fs path")
	flag.StringVar(&dc.BpfConfig.Cgroup2Path, "cgroup2Path", "/mnt/cgroup2/", "cgroup2 path")

	flag.StringVar(&dc.ClientConfig.ClientMode, "clientMode", ClientModeKube, "controller plane mode")
	flag.BoolVar(&dc.ClientConfig.KubeInCluster,"kubeInCluster", false, "deploy in kube cluster")

	flag.Parse()
	fmt.Println(config)
	return nil
}

func (dc *DaemonConfig) String() string {
	return fmt.Sprintf("%#v", *dc)
}

func GetBpfConfig() BpfConfig {
	return config.BpfConfig
}

func GetClientConfig() ClientConfig {
	return config.ClientConfig
}

func EnabledProtocolConfig(s string) bool {
	return config.Protocol[s]
}