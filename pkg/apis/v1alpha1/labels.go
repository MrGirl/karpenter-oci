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
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
)

func init() {
	v1beta1.RestrictedLabelDomains = v1beta1.RestrictedLabelDomains.Insert(RestrictedLabelDomains...)
	v1beta1.WellKnownLabels = v1beta1.WellKnownLabels.Insert(
		LabelInstanceShapeName,
		LabelInstanceCPU,
		LabelInstanceGPU,
		LabelInstanceGPUDescription,
		LabelInstanceMemory,
		LabelInstanceNetworkBandwidth,
		LabelInstanceMaxVNICs,
		LabelIsFlexible,
	)
}

var (
	RestrictedLabelDomains = []string{
		Group,
	}

	LabelNodeClass = Group + "/ocinodeclass"

	LabelInstanceShapeName        = Group + "/instance-shape-name"
	LabelInstanceCPU              = Group + "/instance-cpu"
	LabelInstanceMemory           = Group + "/instance-memory"
	LabelInstanceGPU              = Group + "/instance-gpu"
	LabelInstanceGPUDescription   = Group + "/instance-gpu-description"
	LabelInstanceNetworkBandwidth = Group + "/instance-network-bandwidth"
	LabelInstanceMaxVNICs         = Group + "/instance-max-vnics"
	LabelIsFlexible               = Group + "/is-flexible"

	AnnotationOciNodeClassHash        = Group + "/ocinodeclass-hash"
	AnnotationOciNodeClassHashVersion = Group + "/ocinodeclass-hash-version"

	ResourceNVIDIAGPU v1.ResourceName = "nvidia.com/gpu"
)

const (
	Ubuntu2204ImageFamily  = "Ubuntu2204"
	OracleLinuxImageFamily = "OracleLinux"
)
