/*
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

package v1alpha1

import (
	"fmt"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OciNodeClassSpec struct {
	Image              *Image              `json:"image"`
	VcnId              string              `json:"vcnId"`
	SubnetName         string              `json:"subnetName"`
	SecurityGroupNames []string            `json:"securityGroupNames,omitempty"`
	UserData           *string             `json:"userData,omitempty"`
	PreInstallScript   *string             `json:"preInstallScript,omitempty"`
	ImageFamily        string              `json:"imageFamily"`
	Tags               map[string]string   `json:"tags,omitempty"`
	BootConfig         *BootConfig         `json:"bootConfig"`
	LaunchOptions      *LaunchOptions      `json:"launchOptions,omitempty"`
	BlockDevices       []*VolumeAttributes `json:"blockDevices,omitempty"`
	InstanceShapeAds   []string            `json:"instanceShapeAds"`
}

type VolumeAttributes struct {
	SizeInGBs int64 `json:"sizeInGBs"`
	VpusPerGB int64 `json:"vpusPerGB"`
}

type OciNodeClassStatus struct {
	Subnets []*Subnet `json:"subnets,omitempty"`
	Image   *Image    `json:"image,omitempty"`
}

type Subnet struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Image struct {
	Id            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	CompartmentId string `json:"compartmentId,omitempty"`
}

type BootConfig struct {
	BootVolumeSizeInGBs int64 `json:"bootVolumeSizeInGBs"`
	BootVolumeVpusPerGB int64 `json:"bootVolumeVpusPerGB"`
}

type LaunchOptions struct {

	// Emulation type for the boot volume.
	// * `ISCSI` - ISCSI attached block storage device.
	// * `SCSI` - Emulated SCSI disk.
	// * `IDE` - Emulated IDE disk.
	// * `VFIO` - Direct attached Virtual Function storage. This is the default option for local data
	// volumes on platform images.
	// * `PARAVIRTUALIZED` - Paravirtualized disk. This is the default for boot volumes and remote block
	// storage volumes on platform images.
	BootVolumeType string `mandatory:"false" json:"bootVolumeType,omitempty"`

	// Firmware used to boot VM. Select the option that matches your operating system.
	// * `BIOS` - Boot VM using BIOS style firmware. This is compatible with both 32 bit and 64 bit operating
	// systems that boot using MBR style bootloaders.
	// * `UEFI_64` - Boot VM using UEFI style firmware compatible with 64 bit operating systems. This is the
	// default for platform images.
	Firmware string `mandatory:"false" json:"firmware,omitempty"`

	// Emulation type for the physical network interface card (NIC).
	// * `E1000` - Emulated Gigabit ethernet controller. Compatible with Linux e1000 network driver.
	// * `VFIO` - Direct attached Virtual Function network controller. This is the networking type
	// when you launch an instance using hardware-assisted (SR-IOV) networking.
	// * `PARAVIRTUALIZED` - VM instances launch with paravirtualized devices using VirtIO drivers.
	NetworkType string `mandatory:"false" json:"networkType,omitempty"`

	// Emulation type for volume.
	// * `ISCSI` - ISCSI attached block storage device.
	// * `SCSI` - Emulated SCSI disk.
	// * `IDE` - Emulated IDE disk.
	// * `VFIO` - Direct attached Virtual Function storage. This is the default option for local data
	// volumes on platform images.
	// * `PARAVIRTUALIZED` - Paravirtualized disk. This is the default for boot volumes and remote block
	// storage volumes on platform images.
	RemoteDataVolumeType string `mandatory:"false" json:"remoteDataVolumeType,omitempty"`

	// Whether to enable consistent volume naming feature. Defaults to false.
	IsConsistentVolumeNamingEnabled bool `mandatory:"false" json:"isConsistentVolumeNamingEnabled"`
}

// OciNodeClass is the Schema for the OciNodeClass API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ocinodeclasses,scope=Cluster,categories=karpenter,shortName={ocinc,ocincs}
// +kubebuilder:subresource:status
type OciNodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OciNodeClassSpec   `json:"spec,omitempty"`
	Status OciNodeClassStatus `json:"status,omitempty"`
}

// OciNodeClassList contains a list of OciNodeClass
// +kubebuilder:object:root=true
type OciNodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OciNodeClass `json:"items"`
}

// We need to bump the EC2NodeClassHashVersion when we make an update to the EC2NodeClass CRD under these conditions:
// 1. A field changes its default value for an existing field that is already hashed
// 2. A field is added to the hash calculation with an already-set value
// 3. A field is removed from the hash calculations
const OciNodeClassHashVersion = "v1"

func (in *OciNodeClass) Hash() string {
	return fmt.Sprint(lo.Must(hashstructure.Hash(in.Spec, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets:    true,
		IgnoreZeroValue: true,
		ZeroNil:         true,
	})))
}
