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

package subnet_test

import (
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/patrickmn/go-cache"
	"karpenter-oci/pkg/apis/v1alpha1"
	ocicache "karpenter-oci/pkg/cache"
	"karpenter-oci/pkg/providers/subnet"
	"testing"
)

func TestListSubnet(t *testing.T) {
	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(common.CustomProfileSessionTokenConfigProvider("~/.oci/config", "SESSION"))
	provider := subnet.NewProvider(client, cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval))
	if err != nil {
		t.Fail()
		return
	}
	subnets, err := provider.List(context.Background(), &v1alpha1.OciNodeClass{
		Spec: v1alpha1.OciNodeClassSpec{
			SubnetName: "subnetName",
		},
	})
	if err != nil {
		t.Fail()
		return
	}
	for _, subnet := range subnets {
		fmt.Printf("subnet id: %s", *subnet.Id)
	}
}
