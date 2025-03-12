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

package securitygroup_test

import (
	"context"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/samber/lo"
	"karpenter-oci/pkg/apis"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/operator/options"
	"karpenter-oci/pkg/test"
	"karpenter-oci/pkg/utils"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var ociEnv *test.Environment
var nodeClass *v1alpha1.OciNodeClass

func TestOci(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "SecurityGroupProvider")
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

var _ = Describe("SecurityGroupProvider", func() {
	It("should discover security groups by names", func() {
		nodeClass.Spec.SecurityGroupNames = []string{"securityGroup-test2", "securityGroup-test3"}
		securityGroups, err := ociEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]core.NetworkSecurityGroup{
			{
				Id:          common.String("sg-test2"),
				DisplayName: common.String("securityGroup-test2"),
			},
			{
				Id:          common.String("sg-test3"),
				DisplayName: common.String("securityGroup-test3"),
			},
		}, securityGroups)
	})
	Context("Provider Cache", func() {
		It("should resolve security groups from cache that are filtered by name", func() {
			expectedSecurityGroups := ociEnv.VcnCli.ListSecurityGroupOutput.Clone().Items
			for _, sg := range expectedSecurityGroups {
				nodeClass.Spec.SecurityGroupNames = []string{utils.ToString(sg.DisplayName)}
				// Call list to request from oci and store in the cache
				_, err := ociEnv.SecurityGroupProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
			}

			for _, cachedObject := range ociEnv.SecurityGroupCache.Items() {
				cachedSecurityGroup := cachedObject.Object.([]core.NetworkSecurityGroup)
				Expect(cachedSecurityGroup).To(HaveLen(2))
				lo.ContainsBy(expectedSecurityGroups, func(item core.NetworkSecurityGroup) bool {
					return lo.FromPtr(item.Id) == lo.FromPtr(expectedSecurityGroups[0].Id)
				})
			}
		})
	})
})

func ExpectConsistsOfSecurityGroups(expected, actual []core.NetworkSecurityGroup) {
	GinkgoHelper()
	Expect(actual).To(HaveLen(len(expected)))
	for _, elem := range expected {
		_, ok := lo.Find(actual, func(s core.NetworkSecurityGroup) bool {
			return lo.FromPtr(s.Id) == lo.FromPtr(elem.Id) &&
				lo.FromPtr(s.DisplayName) == lo.FromPtr(elem.DisplayName)
		})
		Expect(ok).To(BeTrue(), `Expected security group with {"GroupId": %q, "GroupName": %q} to exist`, lo.FromPtr(elem.Id), lo.FromPtr(elem.DisplayName))
	}
}
