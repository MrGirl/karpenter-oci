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
	"context"
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	"karpenter-oci/pkg/apis"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/cloudprovider"
	"karpenter-oci/pkg/fake"
	"karpenter-oci/pkg/operator/options"
	"karpenter-oci/pkg/providers/instancetype"
	"karpenter-oci/pkg/test"
	. "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sort"
	"testing"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var ociEnv *test.Environment
var fakeClock *clock.FakeClock
var prov *provisioning.Provisioner
var cluster *state.Cluster
var cloudProvider *cloudprovider.CloudProvider

func TestInstanceType(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceTypeProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ociEnv = test.NewEnvironment(ctx, env)
	fakeClock = &clock.FakeClock{}
	cloudProvider = cloudprovider.New(ociEnv.InstanceTypesProvider, ociEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, ociEnv.AMIProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, events.NewRecorder(&record.FakeRecorder{}), cloudProvider, cluster)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	cluster.Reset()
	ociEnv.Reset()
	ociEnv.LaunchTemplateProvider.ClusterEndpoint = "https://test-cluster"
})

var _ = AfterEach(func() {
	test.ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("InstanceTypeProvider", func() {
	var nodeClass *v1alpha1.OciNodeClass
	var nodePool *corev1beta1.NodePool
	BeforeEach(func() {
		nodeClass = test.OciNodeClass()
		nodePool = coretest.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Template: corev1beta1.NodeClaimTemplate{
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

	It("should support individual instance type labels", func() {
		test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)

		nodeSelector := map[string]string{
			// Well known
			corev1beta1.NodePoolLabelKey:    nodePool.Name,
			v1.LabelTopologyZone:            "US-ASHBURN-AD-1",
			v1.LabelTopologyRegion:          "us-ashburn-1",
			v1.LabelInstanceTypeStable:      "shape-2",
			v1.LabelOSStable:                "linux",
			v1.LabelArchStable:              "amd",
			v1.LabelFailureDomainBetaZone:   "US-ASHBURN-AD-1",
			v1.LabelFailureDomainBetaRegion: "us-ashburn-1",
			"beta.kubernetes.io/arch":       "amd",
			"beta.kubernetes.io/os":         "linux",
			v1.LabelInstanceType:            "shape-2",

			corev1beta1.CapacityTypeLabelKey: "on-demand",
			// Well Known to OCI
			v1alpha1.LabelInstanceShapeName:        "shape-2",
			v1alpha1.LabelInstanceCPU:              "2",
			v1alpha1.LabelInstanceMemory:           "8192",
			v1alpha1.LabelInstanceNetworkBandwidth: "10",
			v1alpha1.LabelInstanceMaxVNICs:         "1",
		}

		// Ensure that we're exercising all well known labels
		Expect(lo.Keys(nodeSelector)).To(ContainElements(append(corev1beta1.WellKnownLabels.Difference(sets.New(
			v1.LabelWindowsBuild)).UnsortedList(), lo.Keys(corev1beta1.NormalizedLabels)...)))

		var pods []*v1.Pod
		for key, value := range nodeSelector {
			pods = append(pods, coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{key: value}}))
		}
		test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			test.ExpectScheduled(ctx, env.Client, pod)
		}
	})
	It("should order the instance types by price and only consider the cheapest ones", func() {
		instances := fake.MakeInstances()
		lo.ForEach(instances, func(item *instancetype.WrapShape, index int) {
			ociEnv.CmpCli.DescribeInstanceTypesOutput.Add(item)
		})

		test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
			},
		})
		test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		test.ExpectScheduled(ctx, env.Client, pod)
		its, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).To(BeNil())
		// Order all the instances by their price
		// We need some way to deterministically order them if their prices match
		reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(nodePool.Spec.Template.Spec.Requirements...)
		sort.Slice(its, func(i, j int) bool {
			iPrice := its[i].Offerings.Compatible(reqs).Cheapest().Price
			jPrice := its[j].Offerings.Compatible(reqs).Cheapest().Price
			if iPrice == jPrice {
				return its[i].Name < its[j].Name
			}
			return iPrice < jPrice
		})
		// Expect that the launch template overrides gives the 100 cheapest instance types
		expected := sets.NewString(lo.Map(its[:3], func(i *corecloudprovider.InstanceType, _ int) string {
			return i.Name
		})...)
		Expect(ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.Len()).To(Equal(1))
		call := ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.Pop()

		Expect(expected.Has(*call.Shape)).To(BeTrue(), fmt.Sprintf("expected %s to exist in set", *call.Shape))
	})
	It("should not launch instances w/ instance storage for ephemeral storage resource requests when exceeding blockDeviceMapping", func() {
		test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod(coretest.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceEphemeralStorage: resource.MustParse("5000Gi")},
			},
		})
		test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		test.ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should not set pods to 110", func() {
		instanceInfo, err := ociEnv.InstanceTypesProvider.ListInstanceType(ctx, nodeClass)
		Expect(err).To(BeNil())
		for _, info := range instanceInfo {
			it := instancetype.NewInstanceType(ctx,
				info,
				nodeClass,
				nodePool.Spec.Template.Spec.Kubelet,
				"us-ashburn-1",
				[]string{"us-east-1"},
				ociEnv.InstanceTypesProvider.CreateOfferings(lo.FromPtr(info.Shape.Shape), sets.New[string]("us-east-1"), info.CalcCpu),
			)
			Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 110))
		}
	})
	Context("Metrics", func() {
		It("should expose vcpu metrics for instance types", func() {
			instanceTypes, err := ociEnv.InstanceTypesProvider.List(ctx, nodePool.Spec.Template.Spec.Kubelet, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))
			for _, it := range instanceTypes {
				metric, ok := test.FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_cpu_cores", map[string]string{
					"instance_type": it.Name,
				})
				Expect(ok).To(BeTrue())
				Expect(metric).To(Not(BeNil()))
				value := metric.GetGauge().Value
				Expect(*value).To(BeNumerically(">", 0))
			}
		})
		It("should expose memory metrics for instance types", func() {
			instanceTypes, err := ociEnv.InstanceTypesProvider.List(ctx, nodePool.Spec.Template.Spec.Kubelet, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))
			for _, it := range instanceTypes {
				metric, ok := test.FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_memory_bytes", map[string]string{
					"instance_type": it.Name,
				})
				Expect(ok).To(BeTrue())
				Expect(metric).To(Not(BeNil()))
				value := metric.GetGauge().Value
				Expect(*value).To(BeNumerically(">", 0))
			}
		})
		It("should expose availability metrics for instance types", func() {
			instanceTypes, err := ociEnv.InstanceTypesProvider.List(ctx, nodePool.Spec.Template.Spec.Kubelet, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))
			for _, it := range instanceTypes {
				for _, of := range it.Offerings {
					metric, ok := test.FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_offering_available", map[string]string{
						"instance_type": it.Name,
						"capacity_type": of.CapacityType,
						"zone":          of.Zone,
					})
					Expect(ok).To(BeTrue())
					Expect(metric).To(Not(BeNil()))
					value := metric.GetGauge().Value
					Expect(*value).To(BeNumerically("==", lo.Ternary(of.Available, 1, 0)))
				}
			}
		})
	})

	Context("Overhead", func() {
		var info *instancetype.WrapShape

		BeforeEach(func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				ClusterName: lo.ToPtr("karpenter-cluster"),
			}))

			var ok bool
			instanceInfo, err := ociEnv.InstanceTypesProvider.ListInstanceType(ctx, nodeClass)
			Expect(err).To(BeNil())
			for _, val := range instanceInfo {
				if *val.Shape.Shape == "shape-1" {
					info = val
					ok = true
				}
			}
			Expect(ok).To(BeTrue())
		})
		Context("System Reserved Resources", func() {
			It("should use defaults when no kubelet is specified", func() {
				it := instancetype.NewInstanceType(
					ctx,
					info,
					nodeClass,
					nodePool.Spec.Template.Spec.Kubelet,
					"us-ashburn-1",
					[]string{"us-east-1"},
					ociEnv.InstanceTypesProvider.CreateOfferings("shape-1", sets.New[string]("us-east-1"), info.CalcCpu),
				)
				Expect(it.Overhead.SystemReserved.Cpu().String()).To(Equal("100m"))
				Expect(it.Overhead.SystemReserved.Memory().String()).To(Equal("100Mi"))
			})
			It("should override system reserved cpus when specified", func() {
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					SystemReserved: map[string]string{
						string(v1.ResourceCPU):              "2",
						string(v1.ResourceMemory):           "20Gi",
						string(v1.ResourceEphemeralStorage): "10Gi",
					},
				}
				it := instancetype.NewInstanceType(
					ctx,
					info,
					nodeClass,
					nodePool.Spec.Template.Spec.Kubelet,
					"us-ashburn-1",
					[]string{"us-east-1"},
					ociEnv.InstanceTypesProvider.CreateOfferings("shape-1", sets.New[string]("us-east-1"), info.CalcCpu),
				)
				Expect(it.Overhead.SystemReserved.Cpu().String()).To(Equal("2"))
				Expect(it.Overhead.SystemReserved.Memory().String()).To(Equal("20Gi"))
				Expect(it.Overhead.SystemReserved.StorageEphemeral().String()).To(Equal("10Gi"))
			})
		})
		Context("Kube Reserved Resources", func() {
			It("should use defaults when no kubelet is specified", func() {
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{}
				it := instancetype.NewInstanceType(
					ctx,
					info,
					nodeClass,
					nodePool.Spec.Template.Spec.Kubelet,
					"us-ashburn-1",
					[]string{"us-east-1"},
					ociEnv.InstanceTypesProvider.CreateOfferings("shape-1", sets.New[string]("us-east-1"), info.CalcCpu),
				)
				Expect(it.Overhead.KubeReserved.Cpu().String()).To(Equal("70m"))
				Expect(it.Overhead.KubeReserved.Memory().String()).To(Equal("1Gi"))
				Expect(it.Overhead.KubeReserved.StorageEphemeral().String()).To(Equal("1Gi"))
			})
			It("should override kube reserved when specified", func() {
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					SystemReserved: map[string]string{
						string(v1.ResourceCPU):              "1",
						string(v1.ResourceMemory):           "20Gi",
						string(v1.ResourceEphemeralStorage): "1Gi",
					},
					KubeReserved: map[string]string{
						string(v1.ResourceCPU):              "2",
						string(v1.ResourceMemory):           "10Gi",
						string(v1.ResourceEphemeralStorage): "2Gi",
					},
				}
				it := instancetype.NewInstanceType(
					ctx,
					info,
					nodeClass,
					nodePool.Spec.Template.Spec.Kubelet,
					"us-ashburn-1",
					[]string{"us-east-1"},
					ociEnv.InstanceTypesProvider.CreateOfferings("shape-1", sets.New[string]("us-east-1"), info.CalcCpu),
				)
				Expect(it.Overhead.KubeReserved.Cpu().String()).To(Equal("2"))
				Expect(it.Overhead.KubeReserved.Memory().String()).To(Equal("10Gi"))
				Expect(it.Overhead.KubeReserved.StorageEphemeral().String()).To(Equal("2Gi"))
			})
		})
		Context("Eviction Thresholds", func() {
			BeforeEach(func() {
				ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
					VMMemoryOverheadPercent: lo.ToPtr[float64](0),
				}))
			})
			It("should take the default eviction threshold when none is specified", func() {
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{}
				it := instancetype.NewInstanceType(
					ctx,
					info,
					nodeClass,
					nodePool.Spec.Template.Spec.Kubelet,
					"us-ashburn-1",
					[]string{"us-east-1"},
					ociEnv.InstanceTypesProvider.CreateOfferings("shape-1", sets.New[string]("us-east-1"), info.CalcCpu),
				)
				Expect(it.Overhead.EvictionThreshold.Memory().String()).To(Equal("750Mi"))
			})
		})
		It("should set max-pods to user-defined value if specified", func() {
			instanceInfo, err := ociEnv.InstanceTypesProvider.ListInstanceType(ctx, nodeClass)
			Expect(err).To(BeNil())
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				MaxPods: ptr.Int32(10),
			}
			for _, info := range instanceInfo {
				it := instancetype.NewInstanceType(
					ctx,
					info,
					nodeClass,
					nodePool.Spec.Template.Spec.Kubelet,
					"us-ashburn-1",
					[]string{"us-east-1"},
					ociEnv.InstanceTypesProvider.CreateOfferings("shape-1", sets.New[string]("us-east-1"), info.CalcCpu),
				)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", 10))
			}
		})
		It("should override pods-per-core value", func() {
			instanceInfo, err := ociEnv.InstanceTypesProvider.ListInstanceType(ctx, nodeClass)
			Expect(err).To(BeNil())
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				PodsPerCore: ptr.Int32(1),
			}
			for _, info := range instanceInfo {
				it := instancetype.NewInstanceType(
					ctx,
					info,
					nodeClass,
					nodePool.Spec.Template.Spec.Kubelet,
					"us-ashburn-1",
					[]string{"us-east-1"},
					ociEnv.InstanceTypesProvider.CreateOfferings("shape-1", sets.New[string]("us-east-1"), info.CalcCpu),
				)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", info.CalcCpu))
			}
		})
		It("should take the minimum of pods-per-core and max-pods", func() {
			instanceInfo, err := ociEnv.InstanceTypesProvider.ListInstanceType(ctx, nodeClass)
			Expect(err).To(BeNil())
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				PodsPerCore: ptr.Int32(4),
				MaxPods:     ptr.Int32(20),
			}
			for _, info := range instanceInfo {
				it := instancetype.NewInstanceType(
					ctx,
					info,
					nodeClass,
					nodePool.Spec.Template.Spec.Kubelet,
					"us-ashburn-1",
					[]string{"us-east-1"},
					ociEnv.InstanceTypesProvider.CreateOfferings("shape-1", sets.New[string]("us-east-1"), info.CalcCpu),
				)
				Expect(it.Capacity.Pods().Value()).To(BeNumerically("==", lo.Min([]int64{20, info.CalcCpu * 4})))
			}
		})
		It("shouldn't report more resources than are actually available on instances", func() {

			// reset cache
			ociEnv.Reset()
			ociEnv.CmpCli.DescribeInstanceTypesOutput.Add(&instancetype.WrapShape{Shape: core.Shape{Shape: common.String("t4g.small"),
				IsFlexible: common.Bool(false), Ocpus: common.Float32(1), MemoryInGBs: common.Float32(4),
				NetworkingBandwidthInGbps: common.Float32(10), MaxVnicAttachments: common.Int(1)}})
			ociEnv.CmpCli.DescribeInstanceTypesOutput.Add(&instancetype.WrapShape{Shape: core.Shape{Shape: common.String("t4g.medium"),
				IsFlexible: common.Bool(false), Ocpus: common.Float32(2), MemoryInGBs: common.Float32(8),
				NetworkingBandwidthInGbps: common.Float32(10), MaxVnicAttachments: common.Int(1)}})
			ociEnv.CmpCli.DescribeInstanceTypesOutput.Add(&instancetype.WrapShape{Shape: core.Shape{Shape: common.String("t4g.xlarge"),
				IsFlexible: common.Bool(false), Ocpus: common.Float32(4), MemoryInGBs: common.Float32(16),
				NetworkingBandwidthInGbps: common.Float32(10), MaxVnicAttachments: common.Int(1)}})
			ociEnv.CmpCli.DescribeInstanceTypesOutput.Add(&instancetype.WrapShape{Shape: core.Shape{Shape: common.String("m5.large"),
				IsFlexible: common.Bool(false), Ocpus: common.Float32(2), MemoryInGBs: common.Float32(8),
				NetworkingBandwidthInGbps: common.Float32(10), MaxVnicAttachments: common.Int(1)}})

			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			its, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).To(BeNil())

			instanceTypes := map[string]*corecloudprovider.InstanceType{}
			for _, it := range its {
				instanceTypes[it.Name] = it
			}

			for _, tc := range []struct {
				InstanceType string
				// todo this need to verified in actual oci cluster
				// Actual allocatable values as reported by the node from kubelet. You find these
				// by launching the node and inspecting the node status allocatable.
				Memory resource.Quantity
				CPU    resource.Quantity
			}{
				{
					InstanceType: "t4g.small",
					Memory:       resource.MustParse("1408312Ki"),
					CPU:          resource.MustParse("1930m"),
				},
				{
					InstanceType: "t4g.medium",
					Memory:       resource.MustParse("3377496Ki"),
					CPU:          resource.MustParse("1930m"),
				},
				{
					InstanceType: "t4g.xlarge",
					Memory:       resource.MustParse("15136012Ki"),
					CPU:          resource.MustParse("3920m"),
				},
				{
					InstanceType: "m5.large",
					Memory:       resource.MustParse("7220184Ki"),
					CPU:          resource.MustParse("1930m"),
				},
			} {
				it, ok := instanceTypes[tc.InstanceType]
				Expect(ok).To(BeTrue(), fmt.Sprintf("didn't find instance type %q, add to instanceTypeTestData in ./hack/codegen.sh", tc.InstanceType))

				allocatable := it.Allocatable()
				// We need to ensure that our estimate of the allocatable resources <= the value that kubelet reports.  If it's greater,
				// we can launch nodes that can't actually run the pods.
				Expect(allocatable.Memory().AsApproximateFloat64()).To(BeNumerically("<=", tc.Memory.AsApproximateFloat64()),
					fmt.Sprintf("memory estimate for %s was too large, had %s vs %s", tc.InstanceType, allocatable.Memory().String(), tc.Memory.String()))
				Expect(allocatable.Cpu().AsApproximateFloat64()).To(BeNumerically("<=", tc.CPU.AsApproximateFloat64()),
					fmt.Sprintf("CPU estimate for %s was too large, had %s vs %s", tc.InstanceType, allocatable.Cpu().String(), tc.CPU.String()))
			}
		})
	})
	Context("Insufficient Capacity Error Cache", func() {
		It("should launch instances of different type on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
			ociEnv.CmpCli.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: corev1beta1.CapacityTypeOnDemand, InstanceType: "shape-3", Zone: "US-ASHBURN-AD-1"}})
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pods := []*v1.Pod{
				coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "US-ASHBURN-AD-1"},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "US-ASHBURN-AD-1"},
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
				}),
			}
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			// it should've tried to pack them on a single shape-3 then hit an insufficient capacity error
			for _, pod := range pods {
				test.ExpectNotScheduled(ctx, env.Client, pod)
			}
			nodeNames := sets.NewString()
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				node := test.ExpectScheduled(ctx, env.Client, pod)
				nodeNames.Insert(node.Name)
			}
			Expect(nodeNames.Len()).To(Equal(1))
		})
		It("should launch instances in a different zone on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
			ociEnv.CmpCli.InsufficientCapacityPools.Set([]fake.CapacityPool{{CapacityType: corev1beta1.CapacityTypeOnDemand, InstanceType: "shape-2", Zone: "US-ASHBURN-AD-1"}})
			pod := coretest.UnschedulablePod(coretest.PodOptions{
				NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "shape-2"},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				},
			})
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"US-ASHBURN-AD-1"}},
					}},
				},
			}}}
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// it should've tried to pack them in test-zone-1a on a p3.8xlarge then hit insufficient capacity, the next attempt will try test-zone-1b
			test.ExpectNotScheduled(ctx, env.Client, pod)

			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := test.ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(SatisfyAll(
				HaveKeyWithValue(v1.LabelInstanceTypeStable, "shape-2"),
				SatisfyAny(
					HaveKeyWithValue(v1.LabelTopologyZone, "US-ASHBURN-AD-3"),
					HaveKeyWithValue(v1.LabelTopologyZone, "US-ASHBURN-AD-2"),
				)))
		})
		It("should launch smaller instances than optimal if larger instance launch results in Insufficient Capacity Error", func() {
			ociEnv.CmpCli.InsufficientCapacityPools.Set([]fake.CapacityPool{
				{CapacityType: corev1beta1.CapacityTypeOnDemand, InstanceType: "shape-3", Zone: "US-ASHBURN-AD-1"},
			})
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"shape-3", "shape-2"},
				},
			})
			pods := []*v1.Pod{}
			for i := 0; i < 2; i++ {
				pods = append(pods, coretest.UnschedulablePod(coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
					NodeSelector: map[string]string{
						v1.LabelTopologyZone: "US-ASHBURN-AD-1",
					},
				}))
			}
			// Provisions 2 shape-2 instances since shape-3 was insufficient
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				test.ExpectNotScheduled(ctx, env.Client, pod)
			}
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				node := test.ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("shape-2"))
			}
		})
		It("should return all instance types, even though with no offerings due to Insufficient Capacity Error", func() {
			ociEnv.CmpCli.InsufficientCapacityPools.Set([]fake.CapacityPool{
				{CapacityType: corev1beta1.CapacityTypeOnDemand, InstanceType: "shape-3", Zone: "US-ASHBURN-AD-1"},
				{CapacityType: corev1beta1.CapacityTypeOnDemand, InstanceType: "shape-3", Zone: "US-ASHBURN-AD-2"},
				{CapacityType: corev1beta1.CapacityTypeOnDemand, InstanceType: "shape-3", Zone: "US-ASHBURN-AD-3"},
			})
			nodePool.Spec.Template.Spec.Requirements = nil
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"shape-3"},
				},
			},
			)
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, corev1beta1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      corev1beta1.CapacityTypeLabelKey,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"on-demand"},
				},
			})

			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			for _, ct := range []string{corev1beta1.CapacityTypeOnDemand, corev1beta1.CapacityTypeSpot} {
				for _, zone := range []string{"US-ASHBURN-AD-1", "US-ASHBURN-AD-2", "US-ASHBURN-AD-3"} {
					test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
						coretest.UnschedulablePod(coretest.PodOptions{
							ResourceRequirements: v1.ResourceRequirements{
								Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
							},
							NodeSelector: map[string]string{
								corev1beta1.CapacityTypeLabelKey: ct,
								v1.LabelTopologyZone:             zone,
							},
						}))
				}
			}

			ociEnv.InstanceTypeCache.Flush()
			instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
			Expect(err).To(BeNil())
			instanceTypeNames := sets.NewString()
			for _, it := range instanceTypes {
				instanceTypeNames.Insert(it.Name)
				if it.Name == "shape-3" {
					// should have no valid offerings
					Expect(it.Offerings.Available()).To(HaveLen(0))
				}
			}
			Expect(instanceTypeNames.Has("shape-3"))
		})
	})
	Context("CapacityType", func() {
		It("should default to on-demand", func() {
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := test.ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1beta1.CapacityTypeLabelKey, corev1beta1.CapacityTypeOnDemand))
		})
	})
	Context("Ephemeral Storage", func() {
		BeforeEach(func() {
			nodeClass.Spec.BlockDevices = []*v1alpha1.VolumeAttributes{{
				SizeInGBs: 100,
				VpusPerGB: 20},
			}
		})
		It("should default to EBS defaults when volumeSize is not defined in blockDeviceMappings for AL2 Root volume", func() {
			test.ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			test.ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := test.ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Capacity.StorageEphemeral()).To(Equal(resource.MustParse("100Gi")))
			Expect(ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
			ociEnv.CmpCli.LaunchInstanceBehavior.CalledWithInput.ForEach(func(ltInput *core.LaunchInstanceRequest) {
				Expect(ltInput.LaunchVolumeAttachments).To(HaveLen(1))
				data, err := json.Marshal(ltInput.LaunchVolumeAttachments[0].GetLaunchCreateVolumeDetails())
				Expect(err).To(BeNil())
				volumeAttr := &core.LaunchCreateVolumeFromAttributes{}
				err = json.Unmarshal(data, volumeAttr)
				Expect(err).To(BeNil())
				Expect(lo.FromPtr(volumeAttr.SizeInGBs)).To(Equal(int64(100)))
				Expect(lo.FromPtr(volumeAttr.VpusPerGB)).To(Equal(int64(20)))
			})
		})
	})
})
