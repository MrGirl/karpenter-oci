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
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"karpenter-oci/pkg/apis"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/cloudprovider"
	"karpenter-oci/pkg/fake"
	"karpenter-oci/pkg/operator/options"
	"karpenter-oci/pkg/test"
	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var env *coretest.Environment
var ociEnv *test.Environment
var cloudProvider *cloudprovider.CloudProvider

func TestInstance(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ociEnv = test.NewEnvironment(ctx, env)
	cloudProvider = cloudprovider.New(ociEnv.InstanceTypesProvider, ociEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, ociEnv.AMIProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ociEnv.Reset()
})

var _ = Describe("InstanceProvider", func() {
	var nodeClass *v1alpha1.OciNodeClass
	var nodePool *v1beta1.NodePool
	var nodeClaim *v1beta1.NodeClaim
	BeforeEach(func() {
		nodeClass = test.OciNodeClass()
		nodePool = coretest.NodePool(v1beta1.NodePool{
			Spec: v1beta1.NodePoolSpec{
				Template: v1beta1.NodeClaimTemplate{
					Spec: v1beta1.NodeClaimSpec{
						NodeClassRef: &v1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
					},
				},
			},
		})
		nodeClaim = coretest.NodeClaim(v1beta1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1beta1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1beta1.NodeClaimSpec{
				NodeClassRef: &v1beta1.NodeClassReference{
					Name: nodeClass.Name,
				},
			},
		})
	})
	It("should return an ICE error when all attempted instance types return an ICE error", func() {
		test.ExpectApplied(ctx, env.Client, nodeClaim, nodePool, nodeClass)
		ociEnv.CmpCli.InsufficientCapacityPools.Set([]fake.CapacityPool{
			{CapacityType: v1beta1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: v1beta1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
		})
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).ToNot(HaveOccurred())

		// Filter down to a single instance type
		instanceTypes = lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool { return i.Name == "m5.xlarge" })

		// Since all the capacity pools are ICEd. This should return back an ICE error
		instance, err := ociEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())
	})
	It("should return all NodePool-owned instances from List", func() {
		ids := sets.New[string]()
		// Provision instances that have the karpenter.sh/nodepool key
		for i := 0; i < 20; i++ {
			instanceID := fake.InstanceID()
			ociEnv.CmpCli.Instances.Store(
				instanceID,
				&core.Instance{
					Id:          common.String(instanceID),
					Shape:       common.String("VM.Standard.E4.Flex"),
					DisplayName: common.String("test-cluster-karpenter"),
					FreeformTags: map[string]string{
						v1beta1.NodePoolLabelKey:       "default",
						v1beta1.ManagedByAnnotationKey: options.FromContext(ctx).ClusterName,
						v1alpha1.LabelNodeClass:        "default"},
					TimeCreated: &common.SDKTime{Time: time.Now().Add(-time.Minute)},
				},
			)
			ids.Insert(instanceID)
		}
		// Provision instances that do not have correct display name
		for i := 0; i < 20; i++ {
			instanceID := fake.InstanceID()
			ociEnv.CmpCli.Instances.Store(
				instanceID,
				&core.Instance{
					Id:          common.String(instanceID),
					Shape:       common.String("VM.Standard.E4.Flex"),
					DisplayName: common.String("test-cluster-karpenter-2"),
					TimeCreated: &common.SDKTime{Time: time.Now().Add(-time.Minute)},
				},
			)
		}
		instances, err := ociEnv.InstanceProvider.List(ctx)
		Expect(err).To(BeNil())
		Expect(instances).To(HaveLen(20))

		retrievedIDs := sets.New[string](lo.Map(instances, func(i core.Instance, _ int) string { return *i.Id })...)
		Expect(ids.Equal(retrievedIDs)).To(BeTrue())
	})
})
