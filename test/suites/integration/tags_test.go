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

package integration_test

import (
	"github.com/zoom/karpenter-oci/pkg/apis/v1alpha1"

	"github.com/awslabs/operatorpkg/object"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/zoom/karpenter-oci/pkg/test"
)

var _ = Describe("Tags", func() {
	Context("Static Tags", func() {
		It("should tag all associated resources", func() {
			// TODO impl
		})
	})
	Context("Tagging Controller", func() {
		It("should tag with karpenter.sh/nodeclaim and Name tag", func() {
			pod := coretest.Pod()
			env.ExpectCreated(nodePool, nodeClass, pod)
			env.EventuallyExpectCreatedNodeCount("==", 1)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			nodeClaim := env.ExpectNodeClaimCount("==", 1)[0]

			nodeInstance := env.GetInstance(node.Name)
			Expect(nodeInstance.DefinedTags).To(HaveKeyWithValue("Name", node.Name))
			Expect(nodeInstance.DefinedTags).To(HaveKeyWithValue("karpenter.sh/nodeclaim", nodeClaim.Name))
			Expect(nodeInstance.DefinedTags).To(HaveKeyWithValue("eks:eks-cluster-name", env.ClusterName))
		})
		It("shouldn't overwrite custom Name tags", func() {
			nodeClass = test.OciNodeClass(*nodeClass, v1alpha1.OciNodeClass{Spec: v1alpha1.OciNodeClassSpec{
				Tags: map[string]string{"Name": "custom-name", "testing/cluster": env.ClusterName},
			}})
			nodePool = coretest.NodePool(*nodePool, karpv1.NodePool{
				Spec: karpv1.NodePoolSpec{
					Template: karpv1.NodeClaimTemplate{
						Spec: karpv1.NodeClaimTemplateSpec{
							NodeClassRef: &karpv1.NodeClassReference{
								Group: object.GVK(nodeClass).Group,
								Kind:  object.GVK(nodeClass).Kind,
								Name:  nodeClass.Name,
							},
						},
					},
				},
			})
			pod := coretest.Pod()
			env.ExpectCreated(nodePool, nodeClass, pod)
			env.EventuallyExpectCreatedNodeCount("==", 1)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			nodeClaim := env.ExpectNodeClaimCount("==", 1)[0]

			nodeInstance := env.GetInstance(node.Name)
			Expect(nodeInstance.DefinedTags).To(HaveKeyWithValue("Name", "custom-name"))
			Expect(nodeInstance.DefinedTags).To(HaveKeyWithValue("karpenter.sh/nodeclaim", nodeClaim.Name))
			Expect(nodeInstance.DefinedTags).To(HaveKeyWithValue("eks:eks-cluster-name", env.ClusterName))
		})
	})
})
