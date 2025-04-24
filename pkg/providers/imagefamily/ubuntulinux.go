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

package imagefamily

import (
	v1 "k8s.io/api/core/v1"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/providers/imagefamily/bootstrap"
)

type UbuntuLinux struct {
	DefaultFamily
	*Options
}

func (a UbuntuLinux) UserData(kubeletConfig *v1alpha1.KubeletConfiguration, taints []v1.Taint, labels map[string]string, customUserData *string, preInstallScript *string) bootstrap.Bootstrapper {
	return bootstrap.Custom{
		Options: bootstrap.Options{
			ClusterName:      a.Options.ClusterName,
			ClusterEndpoint:  a.Options.ClusterEndpoint,
			ClusterDns:       a.Options.ClusterDns,
			CABundle:         a.Options.CABundle,
			BootstrapToken:   a.Options.BootstrapToken,
			KubeletConfig:    kubeletConfig,
			Taints:           taints,
			Labels:           labels,
			CustomUserData:   customUserData,
			PreInstallScript: preInstallScript,
		},
	}
}
