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

package instancetype_test

import (
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	ocicache "karpenter-oci/pkg/cache"
	"karpenter-oci/pkg/providers/instancetype"
	core_resource "sigs.k8s.io/karpenter/pkg/utils/resources"
)

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/patrickmn/go-cache"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/operator/options"
	"testing"
)

func TestListInstanceType(t *testing.T) {
	ctx := context.Background()
	ctx = options.ToContext(ctx, &options.Options{
		VMMemoryOverheadPercent: 0.075,
	})
	providerConfig := common.CustomProfileSessionTokenConfigProvider("~/.oci/config", "SESSION")
	client, err := core.NewComputeClientWithConfigurationProvider(providerConfig)
	region := lo.Must(providerConfig.Region())
	unavailableCache := ocicache.NewUnavailableOfferings()
	provider := instancetype.NewProvider(region, client, cache.New(ocicache.InstanceTypesAndZonesTTL, ocicache.DefaultCleanupInterval), unavailableCache)
	types, err := provider.List(ctx, &v1alpha1.KubeletConfiguration{
		MaxPods:        nil,
		PodsPerCore:    nil,
		SystemReserved: nil,
		KubeReserved:   nil,
	}, &v1alpha1.OciNodeClass{
		Spec: v1alpha1.OciNodeClassSpec{},
	})
	if err != nil {
		t.Fail()
		return
	}
	for _, insType := range types {
		if insType.Name == "VM.Standard.E4.Flex" {
			require, _ := json.Marshal(insType.Requirements)
			capa, _ := json.Marshal(insType.Capacity)
			fmt.Printf("type: %s, requirements: %s, capcity: %s \n", insType.Name, string(require), string(capa))
		}
	}
}

func TestOverHead(t *testing.T) {
	cpus := int64(4) // 4 cores
	memory := 7.0    // 7 GiB
	expectedCPU := "85m"
	expectedMemory := "1Gi"

	resources := instancetype.KubeReservedResources(nil, core_resource.Quantity(fmt.Sprint(cpus)), core_resource.Quantity(fmt.Sprintf("%fGi", memory)))
	gotCPU := resources[v1.ResourceCPU]
	gotMemory := resources[v1.ResourceMemory]

	if gotCPU.String() != expectedCPU {
		t.Fail()
	}
	if gotMemory.String() != expectedMemory {
		t.Fail()
	}

	cpus = int64(2) // 2 cores
	memory = 12.0   // 12 GiB
	expectedCPU = "70m"
	expectedMemory = "1Gi"

	resources = instancetype.KubeReservedResources(nil, core_resource.Quantity(fmt.Sprint(cpus)), core_resource.Quantity(fmt.Sprintf("%fGi", memory)))
	gotCPU = resources[v1.ResourceCPU]
	gotMemory = resources[v1.ResourceMemory]

	if gotCPU.String() != expectedCPU {
		t.Fail()
	}
	if gotMemory.String() != expectedMemory {
		t.Fail()
	}

	cpus = int64(12) // 12 cores
	memory = 160.0   // 160 GiB
	expectedCPU = "107m"
	expectedMemory = "9871Mi"

	resources = instancetype.KubeReservedResources(nil, core_resource.Quantity(fmt.Sprint(cpus)), core_resource.Quantity(fmt.Sprintf("%fGi", memory)))
	gotCPU = resources[v1.ResourceCPU]
	gotMemory = resources[v1.ResourceMemory]

	if gotCPU.String() != expectedCPU {
		t.Fail()
	}
	if gotMemory.String() != expectedMemory {
		t.Fail()
	}

	cpus = int64(8) // 8 cores
	memory = 32     // 32 GiB
	expectedCPU = "97m"
	expectedMemory = "2Gi"

	resources = instancetype.KubeReservedResources(nil, core_resource.Quantity(fmt.Sprint(cpus)), core_resource.Quantity(fmt.Sprintf("%fGi", memory)))
	gotCPU = resources[v1.ResourceCPU]
	gotMemory = resources[v1.ResourceMemory]

	if gotCPU.String() != expectedCPU {
		t.Fail()
	}
	if gotMemory.String() != expectedMemory {
		t.Fail()
	}
}

func TestAllocatable(t *testing.T) {
	memory := 7.0    // 7 GiB
	storage := 100.0 // 100 Gi
	expectedMemory := "750Mi"
	expectStorage := int64(15)
	hardEvict := map[string]string{"memory.available": "750Mi", "nodefs.available": "15%"}
	resources := instancetype.EvictionThreshold(core_resource.Quantity(fmt.Sprintf("%fGi", memory)), core_resource.Quantity(fmt.Sprintf("%fGi", storage)), &v1alpha1.KubeletConfiguration{EvictionHard: hardEvict})
	gotStorage := resources[v1.ResourceEphemeralStorage]
	gotMemory := resources[v1.ResourceMemory]

	if gotStorage.Value()/1024/1024/1024 != expectStorage {
		t.Fail()
	}
	if gotMemory.String() != expectedMemory {
		t.Fail()
	}
}
