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
	"context"
	"fmt"
	"github.com/imdario/mergo"
	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/providers/imagefamily/bootstrap"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

const (
	MemoryAvailable   = "memory.available"
	NodeFSAvailable   = "nodefs.available"
	NodeFSInodesFree  = "nodefs.inodesFree"
	ImageFSAvailable  = "imagefs.available"
	ImageFSInodesFree = "imagefs.inodesFree"
)

type Resolver struct {
	amiProvider *Provider
}

// NewResolver constructs a new launch template Resolver
func NewResolver(amiProvider *Provider) *Resolver {
	return &Resolver{
		amiProvider: amiProvider,
	}
}

// Options define the static launch template parameters
type Options struct {
	ClusterName     string
	ClusterEndpoint string
	ClusterDns      string
	CABundle        *string `hash:"ignore"`
	BootstrapToken  string
	Labels          map[string]string `hash:"ignore"`
	NodeClassName   string
}

// LaunchTemplate holds the dynamically generated launch template parameters
type LaunchTemplate struct {
	*Options
	UserData bootstrap.Bootstrapper
	ImageId  string
}

// DefaultFamily provides default values for AMIFamilies that compose it
type DefaultFamily struct{}

type ImageFamily interface {
	UserData(kubeletConfig *corev1beta1.KubeletConfiguration, taints []core.Taint, labels map[string]string, customUserData *string, preInstallScript *string) bootstrap.Bootstrapper
}

func (r Resolver) Resolve(ctx context.Context, nodeClass *v1alpha1.OciNodeClass, nodeClaim *corev1beta1.NodeClaim, instanceType *cloudprovider.InstanceType, options *Options) ([]*LaunchTemplate, error) {
	images, err := r.amiProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images exist given constraints")
	}
	imageFamily := GetImageFamily(nodeClass.Spec.ImageFamily, options)
	res := make([]*LaunchTemplate, 0)
	for _, image := range images {
		temp, err := r.resolveLaunchTemplate(nodeClass, nodeClaim, instanceType, imageFamily, *image.Id, options)
		if err != nil {
			return nil, err
		}
		res = append(res, temp)
	}
	return res, nil
}

func GetImageFamily(imageFamily string, options *Options) ImageFamily {
	switch imageFamily {
	case v1alpha1.Ubuntu2204ImageFamily:
		return &UbuntuLinux{Options: options}
	case v1alpha1.OracleLinuxImageFamily:
		return &OracleLinux{Options: options}
	default:
		return &OracleLinux{Options: options}
	}
}

func (r Resolver) resolveLaunchTemplate(nodeClass *v1alpha1.OciNodeClass, nodeClaim *corev1beta1.NodeClaim, instanceType *cloudprovider.InstanceType, imageFamily ImageFamily, imageId string, options *Options) (*LaunchTemplate, error) {
	kubeletConfig := nodeClaim.Spec.Kubelet
	if kubeletConfig == nil {
		kubeletConfig = &corev1beta1.KubeletConfiguration{}
	}

	if len(kubeletConfig.KubeReserved) == 0 {
		kubeletConfig.KubeReserved = resources.StringMap(instanceType.Overhead.KubeReserved)
	}
	if len(kubeletConfig.SystemReserved) == 0 {
		kubeletConfig.SystemReserved = resources.StringMap(instanceType.Overhead.SystemReserved)
	}
	if len(kubeletConfig.EvictionHard) == 0 {
		kubeletConfig.EvictionHard = map[string]string{MemoryAvailable: "750Mi", NodeFSAvailable: "10%", NodeFSInodesFree: "5%", ImageFSAvailable: "15%", ImageFSInodesFree: "10%"}
	}
	if kubeletConfig.MaxPods == nil {
		kubeletConfig.MaxPods = lo.ToPtr(int32(instanceType.Capacity.Pods().Value()))
	}

	if nodeClaim.Spec.Kubelet != nil {
		if err := mergo.Merge(kubeletConfig, nodeClaim.Spec.Kubelet); err != nil {
			return nil, err
		}
	}
	resolved := &LaunchTemplate{
		Options: options,
		UserData: imageFamily.UserData(
			kubeletConfig,
			append(nodeClaim.Spec.Taints, nodeClaim.Spec.StartupTaints...),
			options.Labels,
			nodeClass.Spec.UserData,
			nodeClass.Spec.PreInstallScript,
		),
		ImageId: imageId,
	}
	return resolved, nil
}
