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

package instance_test

import (
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"karpenter-oci/pkg/apis/v1alpha1"
	ocicache "karpenter-oci/pkg/cache"
	"karpenter-oci/pkg/operator"
	"karpenter-oci/pkg/operator/options"
	"karpenter-oci/pkg/providers/imagefamily"
	"karpenter-oci/pkg/providers/instance"
	"karpenter-oci/pkg/providers/launchtemplate"
	"karpenter-oci/pkg/providers/securitygroup"
	"karpenter-oci/pkg/providers/subnet"
	"karpenter-oci/pkg/test"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
	"testing"
)

func initProvider(t *testing.T) *instance.Provider {
	ctx = TestContextWithLogger(t)
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	conf := common.CustomProfileSessionTokenConfigProvider("~/.oci/config", "SESSION")
	client, _ := core.NewComputeClientWithConfigurationProvider(conf)
	vncCli, _ := core.NewVirtualNetworkClientWithConfigurationProvider(conf)
	imageProvider := imagefamily.NewProvider(client, cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval))
	imageResolver := imagefamily.NewResolver(imageProvider)
	launchProvider := launchtemplate.NewDefaultProvider(imageResolver, lo.Must(operator.GetCABundle(ctx, nil)), options.FromContext(ctx).ClusterEndpoint, options.FromContext(ctx).BootStrapToken)
	subnetProvider := subnet.NewProvider(vncCli, cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval))
	sgProvider := securitygroup.NewProvider(vncCli, cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval))
	unavailbleCache := ocicache.NewUnavailableOfferings()
	instanceProvider := instance.NewProvider(client, subnetProvider, sgProvider, launchProvider, unavailbleCache)
	return instanceProvider
}

func TestLaunchInstance(t *testing.T) {
	//ctx := context.Background()
	instanceProvider := initProvider(t)
	nodeClass := &v1alpha1.OciNodeClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-karpenter",
		},
		Spec: v1alpha1.OciNodeClassSpec{
			Image: &v1alpha1.Image{
				Name: "imageName",
			},
			SubnetName:   "subnetName",
			UserData:     common.String(""),
			Tags:         map[string]string{"owner": "nathan"},
			BootConfig:   &v1alpha1.BootConfig{BootVolumeSizeInGBs: 50, BootVolumeVpusPerGB: 20},
			BlockDevices: []*v1alpha1.VolumeAttributes{{SizeInGBs: 100, VpusPerGB: 20}},
		},
	}
	ins, err := instanceProvider.Create(ctx, nodeClass, &v1.NodeClaim{}, []*corecloudprovider.InstanceType{})
	if err != nil {
		t.Errorf("failed to create instance, %+v", err)
		t.Fail()
		return
	}
	fmt.Printf("instance id: %s", *ins.Id)
}

func TestDeleteInstance(t *testing.T) {
	ctx := context.Background()
	instanceProvider := initProvider(t)
	err := instanceProvider.Delete(ctx, "instanceID")
	if err != nil {
		t.Fail()
		return
	}
}

func TestDeleteNotExistInstance(t *testing.T) {
	ctx := context.Background()
	instanceProvider := initProvider(t)
	err := instanceProvider.Delete(ctx, "instanceID")
	if err != nil {
		t.Fail()
		return
	}
	err = instanceProvider.Delete(ctx, "instanceID")
	if !corecloudprovider.IsNodeClaimNotFoundError(err) {
		t.Fail()
		return
	}
}

func TestGetInstance(t *testing.T) {
	//ctx := context.Background()
	instanceProvider := initProvider(t)
	instanceID := "ocid1.instance.oc1.iad.anuwcljt6x3ypbyc3vlrugei4kqq7f4nkjccmll4fd2khnpwuspgcfvutlaa"
	ins, err := instanceProvider.Get(ctx, instanceID)
	if err != nil {
		t.Fail()
		return
	}
	fmt.Printf("instance name: %s\n", *ins.DisplayName)

	vnics, err := instanceProvider.GetVnicAttachments(ctx, ins)
	if err != nil {
		t.Errorf("failed to get vnic, %+v\n", err)
		return
	}

	t.Logf("vnic: %+v\n", vnics)

	// get subnets
	subnets, err := instanceProvider.GetSubnets(ctx, vnics)
	if err != nil {
		t.Errorf("failed to get subnets, %+v", subnets)
		return
	}
	t.Logf("subnets: %+v", subnets)
	// get sgs
	groups, err := instanceProvider.GetSecurityGroups(ctx, vnics)
	if err != nil {
		t.Errorf("failed to get security groups, %+v", err)
		return
	}

	t.Logf("sgs: %+v", groups)
}

func TestListInstances(t *testing.T) {
	ctx := context.Background()
	instanceProvider := initProvider(t)
	opt := &options.Options{ClusterName: "clusterName", CompartmentId: "compartmentId", TagNamespace: "common-k8s"}
	ctx = opt.ToContext(ctx)
	inses, err := instanceProvider.List(ctx)
	if err != nil {
		t.Fail()
		return
	}
	for _, ins := range inses {
		if ins.LifecycleState == core.InstanceLifecycleStateTerminated {
			fmt.Printf("ins id: %s, status: %s\n", *ins.Id, ins.LifecycleState)
		}
	}
}
