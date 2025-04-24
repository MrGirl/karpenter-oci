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

package cloudprovider

import (
	"context"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	"karpenter-oci/pkg/apis"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/operator/options"
	"karpenter-oci/pkg/test"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
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
var stop context.CancelFunc
var env *coretest.Environment
var ociEnv *test.Environment
var prov *provisioning.Provisioner
var cloudProvider *CloudProvider
var cluster *state.Cluster
var fakeClock *clock.FakeClock
var recorder events.Recorder

func TestProvider(t *testing.T) {
	ctx = TestContextWithLogger(t)
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "cloudProvider/OCI")
}

var _ = ginkgo.BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	ociEnv = test.NewEnvironment(ctx, env)
	fakeClock = clock.NewFakeClock(time.Now())
	recorder = events.NewRecorder(&record.FakeRecorder{})
	cloudProvider = New(ociEnv.InstanceTypesProvider, ociEnv.InstanceProvider, recorder,
		env.Client, ociEnv.AMIProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, recorder, cloudProvider, cluster)
})

var _ = ginkgo.AfterSuite(func() {
	stop()
	gomega.Expect(env.Stop()).To(gomega.Succeed(), "Failed to stop environment")
})

var _ = ginkgo.BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())

	cluster.Reset()
	ociEnv.Reset()

	ociEnv.LaunchTemplateProvider.ClusterEndpoint = "https://test-cluster"
})

var _ = ginkgo.AfterEach(func() {
	test.ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("CloudProvider", func() {
	var nodeClass *v1alpha1.OciNodeClass
	var nodePool *corev1beta1.NodePool
	var nodeClaim *corev1beta1.NodeClaim
	var _ = BeforeEach(func() {
		nodeClass = test.OciNodeClass()
		nodePool = coretest.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
						Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
							{NodeSelectorRequirement: v1.NodeSelectorRequirement{Key: corev1beta1.CapacityTypeLabelKey, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.CapacityTypeOnDemand}}},
						},
					},
				},
			},
		})
		nodeClaim = coretest.NodeClaim(corev1beta1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{corev1beta1.NodePoolLabelKey: nodePool.Name},
			},
			Spec: corev1beta1.NodeClaimSpec{
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
				},
			},
		})
	})
	It("should return an ICE error when there are no instance types to launch", func() {
		// Specify no instance types and expect to receive a capacity error
		nodeClaim.Spec.Requirements = []corev1beta1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"test-instance-type"},
				},
			},
		}
		test.ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(cloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(cloudProviderNodeClaim).To(BeNil())
	})
	It("should set ImageID in the status field of the nodeClaim", func() {
		test.ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		Expect(cloudProviderNodeClaim.Status.ImageID).ToNot(BeEmpty())
	})
	It("should return NodeClass Hash on the nodeClaim", func() {
		test.ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodeClaim)
		cloudProviderNodeClaim, err := cloudProvider.Create(ctx, nodeClaim)
		Expect(err).To(BeNil())
		Expect(cloudProviderNodeClaim).ToNot(BeNil())
		_, ok := cloudProviderNodeClaim.ObjectMeta.Annotations[v1alpha1.AnnotationOciNodeClassHash]
		Expect(ok).To(BeTrue())
	})
	//Context("instance Tags", func() {
	//	tags := map[string]string{"test_key": "111"}
	//	It("should set context on the CreateFleet request if specified on the NodePool", func() {
	//		nodeClass.Spec.Tags = tags
	//		test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
	//		pod := coretest.UnschedulablePod()
	//		test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
	//		test.ExpectScheduled(ctx, env.Client, pod)
	//		Expect(ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.Len()).To(Equal(1))
	//		createFleetInput := ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.Pop()
	//		Expect(createFleetInput.FreeformTags["test_key"]).To(Equal("111"))
	//	})
	//})
})
