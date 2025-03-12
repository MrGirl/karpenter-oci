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

package nodeclass_test

import (
	"context"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"k8s.io/client-go/tools/record"
	"karpenter-oci/pkg/apis"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/controllers/nodeclass"
	"karpenter-oci/pkg/operator/options"
	"karpenter-oci/pkg/test"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/events"
	corecontroller "sigs.k8s.io/karpenter/pkg/operator/controller"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var env *coretest.Environment
var ociEnv *test.Environment
var nodeClassController corecontroller.Controller

func TestNodeClass(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2NodeClass")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...), coretest.WithFieldIndexers(test.OciNodeClassFieldIndexer(ctx)))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ociEnv = test.NewEnvironment(ctx, env)

	nodeClassController = nodeclass.NewController(env.Client, events.NewRecorder(&record.FakeRecorder{}), ociEnv.AMIProvider, ociEnv.SubnetProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ociEnv.Reset()
})

var _ = AfterEach(func() {
	test.ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClassController", func() {
	var nodeClass *v1alpha1.OciNodeClass
	BeforeEach(func() {
		nodeClass = test.OciNodeClass(v1alpha1.OciNodeClass{
			Spec: v1alpha1.OciNodeClassSpec{
				SubnetName: "private-1",
				Image:      &v1alpha1.Image{Name: "Oracle-Linux-8.9-2024.01.26-0-OKE-1.27.10-679"},
			},
		})
	})
	Context("Subnet Status", func() {
		It("Should resolve a valid selectors for Subnet by name", func() {
			nodeClass.Spec.SubnetName = "private-1"
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = test.ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]*v1alpha1.Subnet{
				{
					Id:   "ocid1.subnet.oc1.iad.aaaaaaaa",
					Name: "private-1",
				},
				{
					Id:   "ocid1.subnet.oc1.iad.aaaaaaab",
					Name: "private-1",
				},
			}))
		})
		It("Should update Subnet status when the Subnet selector gets updated by name", func() {
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = test.ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]*v1alpha1.Subnet{
				{
					Id:   "ocid1.subnet.oc1.iad.aaaaaaaa",
					Name: "private-1",
				},
				{
					Id:   "ocid1.subnet.oc1.iad.aaaaaaab",
					Name: "private-1",
				},
			}))

			nodeClass.Spec.SubnetName = "private-2"
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = test.ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]*v1alpha1.Subnet{
				{
					Id:   "ocid1.subnet.oc1.iad.aaaaaaac",
					Name: "private-2",
				},
			}))
		})
		It("Should not resolve a invalid selectors for Subnet", func() {
			nodeClass.Spec.SubnetName = "invalid"
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileFailed(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = test.ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(BeNil())
		})
		It("Should not resolve a invalid selectors for an updated subnet selector", func() {
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = test.ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(Equal([]*v1alpha1.Subnet{
				{
					Id:   "ocid1.subnet.oc1.iad.aaaaaaaa",
					Name: "private-1",
				},
				{
					Id:   "ocid1.subnet.oc1.iad.aaaaaaab",
					Name: "private-1",
				},
			}))

			nodeClass.Spec.SubnetName = "invalid"
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileFailed(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = test.ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Subnets).To(BeNil())
		})
	})
	Context("AMI Status", func() {
		BeforeEach(func() {
			ociEnv.CmpCli.ListImagesOutput.Set(&core.ListImagesResponse{
				Items: []core.Image{
					{
						Id:             common.String("ocid1.image.oc1.iad.aaaaaaaa"),
						LifecycleState: core.ImageLifecycleStateAvailable,
						DisplayName:    common.String("Oracle-Linux-8.9-2024.01.26-0-OKE-1.27.10-679"),
					},
					{
						Id:             common.String("ocid1.image.oc1.iad.aaaaaaab"),
						LifecycleState: core.ImageLifecycleStateAvailable,
						DisplayName:    common.String("Oracle-Linux-8.9-2024.01.26-0-OKE-1.27.10-680"),
					},
				},
			})
		})
		It("should resolve amiSelector AMIs and requirements into status", func() {
			ociEnv.CmpCli.ListImagesOutput.Set(&core.ListImagesResponse{
				Items: []core.Image{{
					Id:             common.String("ocid1.image.oc1.iad.aaaaaaaa"),
					LifecycleState: core.ImageLifecycleStateAvailable,
					DisplayName:    common.String("Oracle-Linux-8.9-2024.01.26-0-OKE-1.27.10-679")}},
			})
			nodeClass.Spec.Image = &v1alpha1.Image{Name: "Oracle-Linux-8.9-2024.01.26-0-OKE-1.27.10-679"}
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = test.ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Image).To(Equal(&v1alpha1.Image{
				Name: "Oracle-Linux-8.9-2024.01.26-0-OKE-1.27.10-679",
				Id:   "ocid1.image.oc1.iad.aaaaaaaa",
			}))
		})
		It("Should resolve a valid AMI selector", func() {
			nodeClass.Spec.Image = &v1alpha1.Image{Name: "Oracle-Linux-8.9-2024.01.26-0-OKE-1.27.10-680"}
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			nodeClass = test.ExpectExists(ctx, env.Client, nodeClass)
			Expect(nodeClass.Status.Image).To(Equal(
				&v1alpha1.Image{
					Name: "Oracle-Linux-8.9-2024.01.26-0-OKE-1.27.10-680",
					Id:   "ocid1.image.oc1.iad.aaaaaaab",
				}))
		})
	})
	Context("NodeClass Termination", func() {
		It("should not delete the EC2NodeClass until all associated NodeClaims are terminated", func() {
			var nodeClaims []*corev1beta1.NodeClaim
			for i := 0; i < 2; i++ {
				nc := coretest.NodeClaim(corev1beta1.NodeClaim{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
					},
				})
				test.ExpectApplied(ctx, env.Client, nc)
				nodeClaims = append(nodeClaims, nc)
			}
			test.ExpectApplied(ctx, env.Client, nodeClass)
			test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))

			Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
			res := test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(res.RequeueAfter).To(Equal(time.Minute * 10))
			test.ExpectExists(ctx, env.Client, nodeClass)

			// Delete one of the NodeClaims
			// The NodeClass should still not delete
			test.ExpectDeleted(ctx, env.Client, nodeClaims[0])
			res = test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			Expect(res.RequeueAfter).To(Equal(time.Minute * 10))
			test.ExpectExists(ctx, env.Client, nodeClass)

			// Delete the last NodeClaim
			// The NodeClass should now delete
			test.ExpectDeleted(ctx, env.Client, nodeClaims[1])
			test.ExpectReconcileSucceeded(ctx, nodeClassController, client.ObjectKeyFromObject(nodeClass))
			test.ExpectNotFound(ctx, env.Client, nodeClass)
		})
	})
})
