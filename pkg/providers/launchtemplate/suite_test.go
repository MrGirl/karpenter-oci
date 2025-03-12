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

package launchtemplate_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/core"
	"k8s.io/apimachinery/pkg/api/resource"
	"karpenter-oci/pkg/apis"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/cloudprovider"
	"karpenter-oci/pkg/operator/options"
	"karpenter-oci/pkg/test"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var ociEnv *test.Environment
var fakeClock *clock.FakeClock
var prov *provisioning.Provisioner
var cluster *state.Cluster
var cloudProvider *cloudprovider.CloudProvider

func TestLaunchTemplate(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "LaunchTemplateProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	ociEnv = test.NewEnvironment(ctx, env)

	fakeClock = &clock.FakeClock{}
	cloudProvider = cloudprovider.New(ociEnv.InstanceTypesProvider, ociEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, ociEnv.AMIProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, events.NewRecorder(&record.FakeRecorder{}), cloudProvider, cluster)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	cluster.Reset()
	ociEnv.Reset()

	ociEnv.LaunchTemplateProvider.ClusterEndpoint = "https://test-cluster"
	ociEnv.LaunchTemplateProvider.CABundle = lo.ToPtr("ca-bundle")
})

var _ = AfterEach(func() {
	test.ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("LaunchTemplate Provider", func() {
	var nodePool *corev1beta1.NodePool
	var nodeClass *v1alpha1.OciNodeClass
	BeforeEach(func() {
		nodeClass = test.OciNodeClass()
		nodePool = coretest.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Template: corev1beta1.NodeClaimTemplate{
					ObjectMeta: corev1beta1.ObjectMeta{
						// TODO @joinnis: Move this into the coretest.NodePool function
						Labels: map[string]string{coretest.DiscoveryLabel: "unspecified"},
					},
					Spec: corev1beta1.NodeClaimSpec{
						Requirements: []corev1beta1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: v1.NodeSelectorRequirement{
									Key:      corev1beta1.CapacityTypeLabelKey,
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{corev1beta1.CapacityTypeOnDemand},
								},
							},
						},
						Kubelet: &corev1beta1.KubeletConfiguration{},
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
					},
				},
			},
		})
	})

	Context("Labels", func() {
		It("should apply labels to the node", func() {
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := test.ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKey(v1.LabelOSStable))
			Expect(node.Labels).To(HaveKey(v1.LabelArchStable))
			Expect(node.Labels).To(HaveKey(v1.LabelInstanceTypeStable))
		})
	})
	Context("Tags", func() {
		It("should combine custom tags and static tags", func() {
			nodeClass.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			test.ExpectScheduled(ctx, env.Client, pod)
			Expect(ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.FreeformTags).To(HaveLen(6))

			// tags should be included in instance
			ExpectTags(createFleetInput.FreeformTags, nodeClass.Spec.Tags)
		})
	})
	Context("Block Device Mappings", func() {
		It("should use custom block device mapping", func() {
			nodeClass.Spec.BlockDevices = []*v1alpha1.VolumeAttributes{
				{
					SizeInGBs: 100,
					VpusPerGB: 20,
				},
				{
					SizeInGBs: 50,
					VpusPerGB: 10,
				},
			}
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			test.ExpectScheduled(ctx, env.Client, pod)
			Expect(ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
			ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.ForEach(func(ltInput *core.LaunchInstanceRequest) {
				volumeAttrs, err := GetVolumeDetails(ltInput)
				Expect(err).To(BeNil())
				Expect(lo.FromPtr(volumeAttrs[0].SizeInGBs)).To(Equal(int64(100)))
				Expect(lo.FromPtr(volumeAttrs[1].SizeInGBs)).To(Equal(int64(50)))
			})
		})
	})
	Context("Ephemeral Storage", func() {
		It("should pack pods when a daemonset has an ephemeral-storage request", func() {
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass, test.DaemonSet(
				test.DaemonSetOptions{
					PodOptions: coretest.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"),
								v1.ResourceMemory:           resource.MustParse("1Gi"),
								v1.ResourceEphemeralStorage: resource.MustParse("1Gi")}},
					}},
			))
			pod := coretest.UnschedulablePod()
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			test.ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods with any ephemeral-storage request", func() {
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("1G"),
				}}})
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			test.ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods with large ephemeral-storage request", func() {
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
				}}})
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			test.ExpectScheduled(ctx, env.Client, pod)
		})
		It("should not pack pods if the sum of pod ephemeral-storage and overhead exceeds node capacity", func() {
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("109Gi"),
				}}})
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			test.ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should launch multiple nodes if sum of pod ephemeral-storage requests exceeds a single nodes capacity", func() {
			var nodes []*v1.Node
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pods := []*v1.Pod{
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("55Gi"),
					},
				},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("55Gi"),
					},
				},
				}),
			}
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				nodes = append(nodes, test.ExpectScheduled(ctx, env.Client, pod))
			}
			Expect(nodes).To(HaveLen(2))
		})
		It("should only pack pods with ephemeral-storage requests that will fit on an available node", func() {
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pods := []*v1.Pod{
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					},
				},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("150Gi"),
					},
				},
				}),
			}
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			test.ExpectScheduled(ctx, env.Client, pods[0])
			test.ExpectNotScheduled(ctx, env.Client, pods[1])
		})
		It("should not pack pod if no available instance types have enough storage", func() {
			test.ExpectApplied(ctx, env.Client, nodePool)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("150Gi"),
				},
			},
			})
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			test.ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using the blockdevicemappings from the provider spec when defined", func() {
			nodeClass.Spec.BlockDevices = []*v1alpha1.VolumeAttributes{
				{
					SizeInGBs: 100,
					VpusPerGB: 20,
				},
				{
					SizeInGBs: 50,
					VpusPerGB: 10,
				},
			}
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
				},
			},
			})
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			test.ExpectScheduled(ctx, env.Client, pod)
		})
	})
	// todo userdata test
})

// ExpectTags verifies that the expected tags are a subset of the tags found
func ExpectTags(existingTags map[string]string, expected map[string]string) {
	GinkgoHelper()
	for expKey, expValue := range expected {
		foundValue, ok := existingTags[expKey]
		Expect(ok).To(BeTrue(), fmt.Sprintf("expected to find tag %s in %s", expKey, existingTags))
		Expect(foundValue).To(Equal(expValue))
	}
}

func GetVolumeDetails(request *core.LaunchInstanceRequest) ([]*core.LaunchCreateVolumeFromAttributes, error) {
	res := make([]*core.LaunchCreateVolumeFromAttributes, 0)
	for _, item := range request.LaunchVolumeAttachments {
		data, err := json.Marshal(item.GetLaunchCreateVolumeDetails())
		if err != nil {
			return nil, err
		}
		volumeAttr := &core.LaunchCreateVolumeFromAttributes{}
		err = json.Unmarshal(data, volumeAttr)
		if err != nil {
			return nil, err
		}
		res = append(res, volumeAttr)
	}
	return res, nil
}
