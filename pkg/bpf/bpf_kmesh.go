/*
 * Copyright 2023 The Kmesh Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.

 * Author: nlgwcy
 * Create: 2022-02-26
 */

package bpf

// #cgo pkg-config: bpf api-v2-c
// #include "kmesh/include/tail_call.h"
import "C"
import (
	"os"
	"reflect"
	"strconv"
	"syscall"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"oncn.io/mesh/bpf/kmesh/bpf2go"
)

type BpfTracePoint struct {
	Info BpfInfo
	Link link.Link
	bpf2go.KmeshTracePointObjects
}

type BpfSockConn struct {
	Info BpfInfo
	Link link.Link
	bpf2go.KmeshCgroupSockObjects
}

type BpfSockOps struct {
	Info BpfInfo
	Link link.Link
	bpf2go.KmeshSockopsObjects
	bpf2go.KmeshFilterObjects
	bpf2go.KmeshRouteConfigObjects
	bpf2go.KmeshClusterObjects
}

type BpfKmesh struct {
	TracePoint BpfTracePoint
	SockConn BpfSockConn
	SockOps  BpfSockOps
}

func (sc *BpfTracePoint) NewBpf(cfg *Config) {
	sc.Info.Config = *cfg
}

func (sc *BpfSockOps) NewBpf(cfg *Config) error {
	sc.Info.Config = *cfg
	sc.Info.BpfFsPath += "/bpf_kmesh/"
	sc.Info.MapPath = sc.Info.BpfFsPath + "map/"

	if err := os.MkdirAll(sc.Info.MapPath,
		syscall.S_IRUSR|syscall.S_IWUSR|syscall.S_IXUSR|
			syscall.S_IRGRP|syscall.S_IXGRP); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func (sc *BpfSockConn) NewBpf(cfg *Config) error {
	sc.Info.Config = *cfg
	sc.Info.BpfFsPath += "/bpf_kmesh/"
	sc.Info.MapPath = sc.Info.BpfFsPath + "map/"

	if err := os.MkdirAll(sc.Info.MapPath,
		syscall.S_IRUSR|syscall.S_IWUSR|syscall.S_IXUSR|
			syscall.S_IRGRP|syscall.S_IXGRP); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func NewBpfKmesh(cfg *Config) (BpfKmesh, error) {
	var err error

	sc := BpfKmesh{}

	sc.TracePoint.NewBpf(cfg)

	if err = sc.SockOps.NewBpf(cfg); err != nil {
		return sc, err
	}

	if err = sc.SockConn.NewBpf(cfg); err != nil {
		return sc, err
	}
	return sc, nil
}

func setInnerMap(spec *ebpf.CollectionSpec) {
	var (
		InnerMapKeySize    uint32 = 4
		InnerMapDataLength uint32 = 1300
		InnerMapMaxEntries uint32 = 1
	)
	for _, v := range spec.Maps {
		if v.Name == "outer_map" {
			v.InnerMap = &ebpf.MapSpec{
				Type:       ebpf.Array,
				KeySize:    InnerMapKeySize,
				ValueSize:  InnerMapDataLength, // C.BPF_INNER_MAP_DATA_LEN
				MaxEntries: InnerMapMaxEntries,
			}
		}
	}
}

func (sc *BpfTracePoint) loadKmeshTracePointObjects() (*ebpf.CollectionSpec, error) {
	var (
		err error
		spec *ebpf.CollectionSpec
		opts ebpf.CollectionOptions
	)
	opts.Programs.LogSize = sc.Info.BpfVerifyLogSize

	spec, err = bpf2go.LoadKmeshTracePoint()
	if err != nil || spec == nil {
		return nil, err
	}

	for _, v := range spec.Programs {
		if v.Name == "connect_ret" {
			v.Type = ebpf.RawTracepointWritable
		}
	}

	if err = spec.LoadAndAssign(&sc.KmeshTracePointObjects, &opts); err !=nil {
		return nil, err
	}

	return spec, nil
}

func (sc *BpfTracePoint) LoadTracePoint() error {
	if _, err := sc.loadKmeshTracePointObjects(); err != nil {
		return err
	}
	return nil
}

func (sc *BpfSockOps) loadKmeshSockopsObjects() (*ebpf.CollectionSpec, error) {
	var (
		err  error
		spec *ebpf.CollectionSpec
		opts ebpf.CollectionOptions
	)
	opts.Maps.PinPath = sc.Info.MapPath
	opts.Programs.LogSize = sc.Info.BpfVerifyLogSize

	spec, err = bpf2go.LoadKmeshSockops()

	if err != nil || spec == nil {
		return nil, err
	}

	setInnerMap(spec)
	setMapPinType(spec, ebpf.PinByName)
	if err = spec.LoadAndAssign(&sc.KmeshSockopsObjects, &opts); err != nil {
		return nil, err
	}

	value := reflect.ValueOf(sc.KmeshSockopsObjects.KmeshSockopsPrograms)
	if err = pinPrograms(&value, sc.Info.BpfFsPath); err != nil {
		return nil, err
	}

	return spec, nil
}

func (sc *BpfSockOps) loadKmeshFilterObjects() (*ebpf.CollectionSpec, error) {
	var (
		err  error
		spec *ebpf.CollectionSpec
		opts ebpf.CollectionOptions
	)
	opts.Maps.PinPath = sc.Info.MapPath
	opts.Programs.LogSize = sc.Info.BpfVerifyLogSize

	spec, err = bpf2go.LoadKmeshFilter()

	if err != nil || spec == nil {
		return nil, err
	}

	setMapPinType(spec, ebpf.PinByName)
	setProgBpfType(spec, sc.Info.Type, sc.Info.AttachType)
	if err = spec.LoadAndAssign(&sc.KmeshFilterObjects, &opts); err != nil {
		return nil, err
	}

	value := reflect.ValueOf(sc.KmeshFilterObjects.KmeshFilterPrograms)
	if err = pinPrograms(&value, sc.Info.BpfFsPath); err != nil {
		return nil, err
	}

	err = sc.KmeshFilterObjects.KmeshFilterMaps.KmeshTailCallProg.Update(
		uint32(C.KMESH_TAIL_CALL_FILTER_CHAIN),
		uint32(sc.KmeshFilterObjects.KmeshFilterPrograms.FilterChainManager.FD()),
		ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	err = sc.KmeshFilterObjects.KmeshFilterMaps.KmeshTailCallProg.Update(
		uint32(C.KMESH_TAIL_CALL_FILTER),
		uint32(sc.KmeshFilterObjects.KmeshFilterPrograms.FilterManager.FD()),
		ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func (sc *BpfSockOps) loadRouteConfigObjects() (*ebpf.CollectionSpec, error) {
	var (
		err  error
		spec *ebpf.CollectionSpec
		opts ebpf.CollectionOptions
	)
	opts.Maps.PinPath = sc.Info.MapPath
	opts.Programs.LogSize = sc.Info.BpfVerifyLogSize

	spec, err = bpf2go.LoadKmeshRouteConfig()

	if err != nil || spec == nil {
		return nil, err
	}

	setMapPinType(spec, ebpf.PinByName)
	setProgBpfType(spec, sc.Info.Type, sc.Info.AttachType)
	if err = spec.LoadAndAssign(&sc.KmeshRouteConfigObjects, &opts); err != nil {
		return nil, err
	}

	value := reflect.ValueOf(sc.KmeshRouteConfigObjects.KmeshRouteConfigPrograms)
	if err = pinPrograms(&value, sc.Info.BpfFsPath); err != nil {
		return nil, err
	}

	err = sc.KmeshRouteConfigObjects.KmeshRouteConfigMaps.KmeshTailCallProg.Update(
		uint32(C.KMESH_TAIL_CALL_ROUTER_CONFIG),
		uint32(sc.KmeshRouteConfigObjects.KmeshRouteConfigPrograms.RouteConfigManager.FD()),
		ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func (sc *BpfSockOps) loadKmeshClusterObjects() (*ebpf.CollectionSpec, error) {
	var (
		err  error
		spec *ebpf.CollectionSpec
		opts ebpf.CollectionOptions
	)
	opts.Maps.PinPath = sc.Info.MapPath
	opts.Programs.LogSize = sc.Info.BpfVerifyLogSize

	spec, err = bpf2go.LoadKmeshCluster()

	if err != nil || spec == nil {
		return nil, err
	}

	setMapPinType(spec, ebpf.PinByName)
	setProgBpfType(spec, sc.Info.Type, sc.Info.AttachType)
	if err = spec.LoadAndAssign(&sc.KmeshClusterObjects, &opts); err != nil {
		return nil, err
	}

	value := reflect.ValueOf(sc.KmeshClusterObjects.KmeshClusterPrograms)
	if err = pinPrograms(&value, sc.Info.BpfFsPath); err != nil {
		return nil, err
	}

	err = sc.KmeshClusterObjects.KmeshClusterMaps.KmeshTailCallProg.Update(
		uint32(C.KMESH_TAIL_CALL_CLUSTER),
		uint32(sc.KmeshClusterObjects.KmeshClusterPrograms.ClusterManager.FD()),
		ebpf.UpdateAny)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func (sc *BpfSockOps) LoadSockOps() error {
	/* load kmesh sockops main bpf prog */
	spec, err := sc.loadKmeshSockopsObjects()
	if err != nil {
		return err
	}

	prog := spec.Programs["sockops_prog"]
	sc.Info.Type = prog.Type
	sc.Info.AttachType = prog.AttachType

	/* load kmesh sockops tail call bpf prog */
	if _, err := sc.loadKmeshFilterObjects(); err != nil {
		return err
	}

	if _, err := sc.loadRouteConfigObjects(); err != nil {
		return err
	}

	if _, err := sc.loadKmeshClusterObjects(); err != nil {
		return err
	}

	return nil
}

func (sc *BpfSockConn) loadKmeshSockConnObjects() (*ebpf.CollectionSpec, error) {
	var (
		err  error
		spec *ebpf.CollectionSpec
		opts ebpf.CollectionOptions
	)
	opts.Maps.PinPath = sc.Info.MapPath
	opts.Programs.LogSize = sc.Info.BpfVerifyLogSize

	spec, err = bpf2go.LoadKmeshCgroupSock()

	if err != nil || spec == nil {
		return nil, err
	}

	setInnerMap(spec)
	setMapPinType(spec, ebpf.PinByName)
	if err = spec.LoadAndAssign(&sc.KmeshCgroupSockObjects, &opts); err != nil {
		return nil, err
	}

	value := reflect.ValueOf(sc.KmeshCgroupSockObjects.KmeshCgroupSockPrograms)
	if err = pinPrograms(&value, sc.Info.BpfFsPath); err != nil {
		return nil, err
	}

	return spec, nil
}

func (sc *BpfSockConn) LoadSockConn() error {
	/* load kmesh sockops main bpf prog */
	spec, err := sc.loadKmeshSockConnObjects()
	if err != nil {
		return err
	}

	prog := spec.Programs["cgroup_connect4_prog"]
	sc.Info.Type = prog.Type
	sc.Info.AttachType = prog.AttachType

	return nil
}

func (sc *BpfKmesh) Load() error {
	var err error

	if err = sc.TracePoint.LoadTracePoint(); err != nil {
		return err
	}

	if err = sc.SockOps.LoadSockOps(); err != nil {
		return err
	}

	if err = sc.SockConn.LoadSockConn(); err != nil {
		return err
	}

	return nil
}

func (sc *BpfKmesh) ApiEnvCfg() error {
	var err error
	var info *ebpf.MapInfo
	var id ebpf.MapID
	info, err = Obj.Kmesh.SockOps.KmeshSockopsMaps.KmeshListener.Info()

	if err != nil {
		return err
	}

	id, _ = info.ID()
	stringId := strconv.Itoa(int(id))
	if err = os.Setenv("Listener", stringId); err != nil {
		return err
	}

	info, _ = Obj.Kmesh.SockOps.KmeshSockopsMaps.OuterMap.Info()
	id, _ = info.ID()
	stringId = strconv.Itoa(int(id))
	if err = os.Setenv("OUTTER_MAP_ID", stringId); err != nil {
		return err
	}

	info, _ = Obj.Kmesh.SockOps.KmeshSockopsMaps.InnerMap.Info()
	id, _ = info.ID()
	stringId = strconv.Itoa(int(id))
	if err = os.Setenv("INNER_MAP_ID", stringId); err != nil {
		return err
	}

	info, _ = Obj.Kmesh.SockOps.MapOfRouterConfig.Info()
	id, _ = info.ID()
	stringId = strconv.Itoa(int(id))
	if err = os.Setenv("RouteConfiguration", stringId); err != nil {
		return err
	}

	info, _ = Obj.Kmesh.SockOps.KmeshCluster.Info()
	id, _ = info.ID()
	stringId = strconv.Itoa(int(id))
	if err = os.Setenv("Cluster", stringId); err != nil {
		return err
	}

	return nil
}

func (sc *BpfTracePoint) Attach() error {
	tpopt := link.RawTracepointOptions {
		Name:    "connect_ret",
		Program: sc.KmeshTracePointObjects.ConnectRet,
	}

	lk, err := link.AttachRawTracepoint(tpopt)
	if err != nil {
		return err
	}
	sc.Link = lk

	return nil
}

func (sc *BpfSockOps) Attach() error {
	cgopt := link.CgroupOptions{
		Path:    sc.Info.Cgroup2Path,
		Attach:  sc.Info.AttachType,
		Program: sc.KmeshSockopsObjects.SockopsProg,
	}

	lk, err := link.AttachCgroup(cgopt)
	if err != nil {
		return err
	}
	sc.Link = lk

	return nil
}

func (sc *BpfSockConn) Attach() error {
	cgopt := link.CgroupOptions{
		Path:    sc.Info.Cgroup2Path,
		Attach:  sc.Info.AttachType,
		Program: sc.KmeshCgroupSockObjects.CgroupConnect4Prog,
	}

	lk, err := link.AttachCgroup(cgopt)
	if err != nil {
		return err
	}
	sc.Link = lk

	return nil
}

func (sc *BpfKmesh) Attach() error {
	var err error

	if err = sc.TracePoint.Attach(); err != nil {
		return err
	}

	if err = sc.SockOps.Attach(); err != nil {
		return err
	}

	if err = sc.SockConn.Attach(); err != nil {
		return err
	}
	return nil
}

func (sc *BpfTracePoint) close() error {
	return sc.KmeshTracePointObjects.Close()
}

func (sc *BpfSockOps) close() error {
	if err := sc.KmeshSockopsObjects.Close(); err != nil {
		return err
	}
	if err := sc.KmeshFilterObjects.Close(); err != nil {
		return err
	}
	if err := sc.KmeshRouteConfigObjects.Close(); err != nil {
		return err
	}
	if err := sc.KmeshClusterObjects.Close(); err != nil {
		return err
	}

	return nil
}

func (sc *BpfSockConn) close() error {
	if err := sc.KmeshCgroupSockObjects.Close(); err != nil {
		return err
	}
	return nil
}

func (sc *BpfKmesh) close() error {
	var err error

	if err = sc.SockOps.close(); err != nil {
		return err
	}

	if err = sc.SockConn.close(); err != nil {
		return err
	}

	if err = sc.TracePoint.close(); err != nil {
		return err
	}
	return nil
}

func (sc *BpfTracePoint) Detach() error {
	if err := sc.close(); err != nil {
		return err
	}

	if sc.Link != nil {
		return sc.Link.Close()
	}
	return nil
}

func (sc *BpfSockOps) Detach() error {
	var value reflect.Value

	if err := sc.close(); err != nil {
		return err
	}

	value = reflect.ValueOf(sc.KmeshSockopsObjects.KmeshSockopsPrograms)
	if err := unpinPrograms(&value); err != nil {
		return err
	}
	value = reflect.ValueOf(sc.KmeshSockopsObjects.KmeshSockopsMaps)
	if err := unpinMaps(&value); err != nil {
		return err
	}

	if err := os.RemoveAll(sc.Info.BpfFsPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if sc.Link != nil {
		return sc.Link.Close()
	}
	return nil
}

func (sc *BpfSockConn) Detach() error {
	var value reflect.Value

	if err := sc.close(); err != nil {
		return err
	}

	value = reflect.ValueOf(sc.KmeshCgroupSockObjects.KmeshCgroupSockPrograms)
	if err := unpinPrograms(&value); err != nil {
		return err
	}
	value = reflect.ValueOf(sc.KmeshCgroupSockObjects.KmeshCgroupSockMaps)
	if err := unpinMaps(&value); err != nil {
		return err
	}

	if err := os.RemoveAll(sc.Info.BpfFsPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if sc.Link != nil {
		return sc.Link.Close()
	}
	return nil
}

func (sc *BpfKmesh) Detach() error {
	var err error

	if err = sc.TracePoint.Detach(); err != nil {
		return err
	}

	if err = sc.SockOps.Detach(); err != nil {
		return err
	}

	if err = sc.SockConn.Detach(); err != nil {
		return err
	}
	return nil
}
