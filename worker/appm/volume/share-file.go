// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package volume

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
)

// ShareFileVolume nfs volume struct
type ShareFileVolume struct {
	Base
}

// CreateVolume share file volume create volume
func (v *ShareFileVolume) CreateVolume(define *Define) error {
	err := util.CheckAndCreateDir(v.svm.HostPath)
	if err != nil {
		return fmt.Errorf("create host path %s error,%s", v.svm.HostPath, err.Error())
	}
	os.Chmod(v.svm.HostPath, 0777)

	volumeMountName := fmt.Sprintf("manual%d", v.svm.ID)
	volumeMountPath := v.svm.VolumePath
	volumeReadOnly := v.svm.IsReadOnly

	var vm *corev1.VolumeMount
	if v.as.GetStatefulSet() != nil {
		statefulset := v.as.GetStatefulSet()
		labels := v.as.GetCommonLabels(map[string]string{"volume_name": volumeMountName})
		annotations := map[string]string{"volume_name": v.svm.VolumeName}
		claim := newVolumeClaim(volumeMountName, volumeMountPath, v.svm.AccessMode, v1.RainbondStatefuleShareStorageClass, v.svm.VolumeCapacity, labels, annotations)
		v.as.SetClaim(claim)
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, *claim)
		vo := corev1.Volume{Name: volumeMountName}
		vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.GetName(), ReadOnly: volumeReadOnly}
		define.volumes = append(define.volumes, vo)
		vm = &corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: volumeMountPath,
			ReadOnly:  volumeReadOnly,
		}
	} else {
		for _, m := range define.volumeMounts {
			if m.MountPath == volumeMountPath { // TODO move to prepare
				logrus.Warningf("found the same mount path: %s, skip it", volumeMountPath)
				return nil
			}
		}
		hostPath := v.svm.HostPath
		if v.as.IsWindowsService {
			hostPath = RewriteHostPathInWindows(hostPath)
		}
		vo := corev1.Volume{Name: volumeMountName}
		hostPathType := corev1.HostPathDirectoryOrCreate
		vo.HostPath = &corev1.HostPathVolumeSource{
			Path: hostPath,
			Type: &hostPathType,
		}
		define.volumes = append(define.volumes, vo)
		vm = &corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: volumeMountPath,
			ReadOnly:  volumeReadOnly,
		}
	}
	if vm != nil {
		define.volumeMounts = append(define.volumeMounts, *vm)
	}

	return nil
}

// CreateDependVolume create depend volume
func (v *ShareFileVolume) CreateDependVolume(define *Define) error {
	volumeMountName := fmt.Sprintf("mnt%d", v.smr.ID)
	volumeMountPath := v.smr.VolumePath
	volumeReadOnly := false
	for _, m := range define.volumeMounts {
		if m.MountPath == volumeMountPath {
			logrus.Warningf("found the same mount path: %s, skip it", volumeMountPath)
			return nil
		}
	}
	err := util.CheckAndCreateDir(v.smr.HostPath)
	if err != nil {
		return fmt.Errorf("create host path %s error,%s", v.smr.HostPath, err.Error())
	}
	hostPath := v.smr.HostPath
	if v.as.IsWindowsService {
		hostPath = RewriteHostPathInWindows(hostPath)
	}

	vo := corev1.Volume{Name: volumeMountName}
	hostPathType := corev1.HostPathDirectoryOrCreate
	vo.HostPath = &corev1.HostPathVolumeSource{
		Path: hostPath,
		Type: &hostPathType,
	}
	define.volumes = append(define.volumes, vo)
	vm := corev1.VolumeMount{
		Name:      volumeMountName,
		MountPath: volumeMountPath,
		ReadOnly:  volumeReadOnly,
	}
	define.volumeMounts = append(define.volumeMounts, vm)
	return nil
}
