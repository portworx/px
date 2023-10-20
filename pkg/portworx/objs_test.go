/*
Copyright © 2019 Portworx

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package portworx

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	api "github.com/libopenstorage/openstorage-sdk-clients/sdk/golang"
	"github.com/portworx/pxc/pkg/util"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

var (
	attachedState = map[string]string{
		"pvc-34d0f15c-65b9-4229-8b3e-b7bb912e382f": "on ip-70-0-87-200.brbnca.spcsdns.net",
		"tp2": "Detached",
		"pvc-6fc1fe2d-25f4-40b0-a616-04c019572154": "on ip-70-0-87-200.brbnca.spcsdns.net",
		"tp3":      "on ip-70-0-87-200.brbnca.spcsdns.net",
		"tp1":      "Detached",
		"tp2-snap": "Detached",
	}

	podToVolume = map[string]string{
		"wordpress-mysql-684ddbbb55-zjs7b": "pvc-6fc1fe2d-25f4-40b0-a616-04c019572154",
		"wordpress-7f6d665c6f-5wpm6":       "pvc-34d0f15c-65b9-4229-8b3e-b7bb912e382f",
		"wordpress-7f6d665c6f-7qcch":       "pvc-34d0f15c-65b9-4229-8b3e-b7bb912e382f",
		"wordpress-7f6d665c6f-ddjj6":       "pvc-34d0f15c-65b9-4229-8b3e-b7bb912e382f",
	}

	volumeToReplicationInfo = map[string]string{
		`tp1`:      `{"Rsi":[{"Id":0,"NodeInfo":["ip-70-0-87-200.brbnca.spcsdns.net (Pool 0)"],"HaIncrease":"","ReAddOn":[]}],"Status":"Detached"}`,
		`tp2`:      `{"Rsi":[{"Id":0,"NodeInfo":["ip-70-0-87-200.brbnca.spcsdns.net (Pool 1)"],"HaIncrease":"","ReAddOn":[]},{"Id":1,"NodeInfo":["ip-70-0-87-203.brbnca.spcsdns.net (Pool 1)"],"HaIncrease":"","ReAddOn":[]}],"Status":"UP"}`,
		`tp2-snap`: `{"Rsi":[{"Id":0,"NodeInfo":["ip-70-0-87-200.brbnca.spcsdns.net (Pool 1)"],"HaIncrease":"","ReAddOn":[]},{"Id":1,"NodeInfo":["ip-70-0-87-203.brbnca.spcsdns.net (Pool 1)"],"HaIncrease":"","ReAddOn":[]}],"Status":"Detached"}`,
		`pvc-6fc1fe2d-25f4-40b0-a616-04c019572154`: `{"Rsi":[{"Id":0,"NodeInfo":["ip-70-0-87-200.brbnca.spcsdns.net (Pool 1)","ip-70-0-87-203.brbnca.spcsdns.net (Pool 1)"],"HaIncrease":"","ReAddOn":[]}],"Status":"UP"}`,
		`tp3`: `{"Rsi":[{"Id":0,"NodeInfo":["ip-70-0-87-233.brbnca.spcsdns.net (Pool 0)"],"HaIncrease":"","ReAddOn":[]},{"Id":1,"NodeInfo":["ip-70-0-87-200.brbnca.spcsdns.net (Pool 1)"],"HaIncrease":"","ReAddOn":[]},{"Id":2,"NodeInfo":["ip-70-0-87-203.brbnca.spcsdns.net (Pool 0)"],"HaIncrease":"","ReAddOn":[]}],"Status":"UP"}`,
		`pvc-34d0f15c-65b9-4229-8b3e-b7bb912e382f`: `{"Rsi":[{"Id":0,"NodeInfo":["ip-70-0-87-200.brbnca.spcsdns.net (Pool 1)","ip-70-0-87-233.brbnca.spcsdns.net (Pool 1)"],"HaIncrease":"","ReAddOn":[]}],"Status":"UP"}`,
	}

	pvcInfo = map[string]string{
		"pvc-6fc1fe2d-25f4-40b0-a616-04c019572154": "mysql-pvc-1",
		"pvc-34d0f15c-65b9-4229-8b3e-b7bb912e382f": "wp-pv-claim",
	}

	expectedNodes = []string{
		"ip-70-0-87-233.brbnca.spcsdns.net",
		"ip-70-0-87-200.brbnca.spcsdns.net",
		"ip-70-0-87-203.brbnca.spcsdns.net",
	}
)

type TestData struct {
	Vols    []*api.SdkVolumeInspectResponse
	Pvcs    []v1.PersistentVolumeClaim
	Pods    []v1.Pod
	NodeMap map[string]*api.StorageNode
}

type testOps struct {
	vols  Volumes
	nodes Nodes
	pods  Pods
	pvcs  Pvcs
}

func testData(t *testing.T) *testOps {
	td := &TestData{}
	err := json.Unmarshal([]byte(dummyInputJson), td)
	assert.Equal(t, err, nil, "Error Unmarshalling string")
	to := &testOps{}
	v := make([]*api.Volume, len(td.Vols))
	for i, vol := range td.Vols {
		v[i] = vol.GetVolume()
	}
	to.vols = &volumes{
		vols: v,
	}

	n := make([]*api.StorageNode, len(td.NodeMap))
	for _, node := range td.NodeMap {
		n = append(n, node)
	}
	to.nodes = &nodes{
		nodeMap: td.NodeMap,
		nodes:   n,
	}

	to.pods = &pods{
		pods: td.Pods,
	}

	to.pvcs = &pvcs{
		pvcs: td.Pvcs,
		pods: to.pods,
		vols: to.vols,
	}

	return to
}

func testAllOps(t *testing.T, to *testOps, v *api.Volume) {
	name := v.GetLocator().GetName()
	state, err := to.nodes.GetAttachedState(v)
	assert.Equal(t, err, nil, "Got error getting attached state")
	expectedState := attachedState[name]
	assert.Equalf(t, state, expectedState, "Attached state is not correct for %s", name)
	pods, err := to.pods.PodsUsingVolume(v)
	assert.Equal(t, err, nil, "Got error getting pods using volume")
	for _, pod := range pods {
		vn := podToVolume[pod.Name]
		assert.Equalf(t, vn, name, "%s should be using %s", pod.Name, name)
	}
	replInfo, err := to.nodes.GetReplicationInfo(v)
	assert.Equal(t, err, nil, "Got error getting replication info")
	ejson := volumeToReplicationInfo[name]
	eReplInfo := &ReplicationInfo{}
	err = json.Unmarshal([]byte(ejson), eReplInfo)
	assert.Equal(t, err, nil, "Got error unmarshalling replication info")
	b := reflect.DeepEqual(replInfo, eReplInfo)
	assert.Equalf(t, b, true, "ReplicationInfo is not same for %s", name)
}

func TestOps(t *testing.T) {
	to := testData(t)
	svols, err := to.vols.GetVolumes()
	assert.Equal(t, err, nil, "Could not get volumes")
	for _, v := range svols {
		testAllOps(t, to, v)
		cinfo, err := to.pods.GetContainerInfoForVolume(v)
		assert.NoError(t, err)
		for _, ci := range cinfo {
			volName := podToVolume[ci.Pod.Name]
			assert.Equal(t, volName, v.GetLocator().GetName())
			assert.Equal(t, ci.Pod.Namespace, "wp1")
			if ci.Container == "mysql" {
				assert.Equal(t, ci.Pod.Name, "wordpress-mysql-684ddbbb55-zjs7b")
			} else {
				assert.Equal(t, ci.Container, "wordpress")
			}
		}
	}

	ns := GetNodeSpec(svols)
	assert.Equal(t, len(expectedNodes), len(ns.NodeNames))
	for _, n := range ns.NodeNames {
		node, err := to.nodes.GetNode(n)
		assert.NoError(t, err)
		assert.True(t, util.ListContains(expectedNodes, node.GetHostname()))
	}

	pxPvcs, err := to.pvcs.GetPxPvcs()
	assert.Equal(t, err, nil, "Got error while trying to get PxPvcs")
	for _, pxPvc := range pxPvcs {
		vname := pxPvc.PxVolume.GetLocator().GetName()
		assert.Equal(t, pxPvc.Name, pxPvc.Pvc.GetName(), "pvc names don't match")
		assert.Equal(t, vname, pxPvc.Pvc.Spec.VolumeName, "Volume name does not match")
		assert.Equal(t, pxPvc.Namespace, pxPvc.Pvc.GetNamespace(), "pxPvc's namespace does not match with pxPvc.Pvc's namespace")
		ePvcName := pvcInfo[vname]
		assert.Equal(t, pxPvc.Name, ePvcName, "pvc names don't match")
		podNames := pxPvc.PodNames
		for _, p := range podNames {
			n := strings.Split(p, "/")
			assert.Equalf(t, pxPvc.Namespace, n[0], "namespace of pod not correct %s", p)
			vn := podToVolume[n[1]]
			assert.Equalf(t, vn, vname, "%s should be using %s", n[1], vname)
		}
	}
}
