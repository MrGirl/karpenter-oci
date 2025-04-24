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

package imagefamily_test

import (
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/patrickmn/go-cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"karpenter-oci/pkg/apis/v1alpha1"
	ocicache "karpenter-oci/pkg/cache"
	"karpenter-oci/pkg/providers/imagefamily"
	"testing"
)

func TestListImage(t *testing.T) {
	client, err := core.NewComputeClientWithConfigurationProvider(common.CustomProfileSessionTokenConfigProvider("~/.oci/config", "SESSION"))
	provider := imagefamily.NewProvider(client, cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval))
	images, err := provider.List(context.Background(), &v1alpha1.OciNodeClass{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1alpha1.OciNodeClassSpec{
			Image: &v1alpha1.Image{
				Name:          "name",
				CompartmentId: "compartmentId",
			},
		},
	})
	if err != nil {
		t.Fail()
		return
	}
	if len(images) != 1 {
		t.Fail()
		return
	}
	for _, image := range images {
		fmt.Printf("image: %s/n", *image.DisplayName)
	}
}
