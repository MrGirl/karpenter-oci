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
	"github.com/oracle/oci-go-sdk/v65/core"
	"karpenter-oci/pkg/apis"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/operator/options"
	"karpenter-oci/pkg/test"
	"testing"

	"github.com/samber/lo"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var ociEnv *test.Environment
var nodeClass *v1alpha1.OciNodeClass

func TestSubnet(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "SubnetProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	ociEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	nodeClass = test.OciNodeClass(v1alpha1.OciNodeClass{
		Spec: v1alpha1.OciNodeClassSpec{},
	})
	ociEnv.Reset()
})

var _ = AfterEach(func() {
	test.ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("SubnetProvider", func() {
	Context("List", func() {
		It("should discover subnet by name", func() {
			nodeClass.Spec.SubnetName = "private-1"
			subnets, err := ociEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]core.Subnet{
				{
					Id:          lo.ToPtr("ocid1.subnet.oc1.iad.aaaaaaaa"),
					DisplayName: lo.ToPtr("private-1"),
				},
				{
					Id:          lo.ToPtr("ocid1.subnet.oc1.iad.aaaaaaab"),
					DisplayName: lo.ToPtr("private-1"),
				},
			}, subnets)
		})
	})
	Context("Provider Cache", func() {
		It("should resolve subnets from cache that are filtered by name", func() {
			expectedSubnets := ociEnv.VcnCli.ListSubnetsOutput.Clone().Items
			for _, subnet := range expectedSubnets {
				nodeClass.Spec.SubnetName = lo.FromPtr[string](subnet.DisplayName)
				// Call list to request from aws and store in the cache
				_, err := ociEnv.SubnetProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
			}

			for _, cachedObject := range ociEnv.SubnetCache.Items() {
				cachedSubnet := cachedObject.Object.([]core.Subnet)
				Expect(cachedSubnet).To(HaveLen(1))
				lo.ContainsBy(expectedSubnets, func(item core.Subnet) bool {
					return lo.FromPtr(item.Id) == lo.FromPtr(cachedSubnet[0].Id)
				})
			}
		})
	})
})

func ExpectConsistsOfSubnets(expected, actual []core.Subnet) {
	GinkgoHelper()
	Expect(actual).To(HaveLen(len(expected)))
	for _, elem := range expected {
		_, ok := lo.Find(actual, func(s core.Subnet) bool {
			return lo.FromPtr(s.Id) == lo.FromPtr(elem.Id) &&
				lo.FromPtr(s.DisplayName) == lo.FromPtr(elem.DisplayName)
		})
		Expect(ok).To(BeTrue(), `Expected subnet with {"SubnetId": %q, "AvailabilityZone": %q} to exist`, lo.FromPtr(elem.Id), lo.FromPtr(elem.DisplayName))
	}
}
