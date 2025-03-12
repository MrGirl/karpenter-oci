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

package test

import (
	"context"
	//nolint:revive,stylecheck
	//nolint:revive,stylecheck
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	apis "karpenter-oci/pkg/apis"
	ocicache "karpenter-oci/pkg/cache"
	fake "karpenter-oci/pkg/fake"
	"karpenter-oci/pkg/providers/imagefamily"
	"karpenter-oci/pkg/providers/instance"
	"karpenter-oci/pkg/providers/instancetype"
	"karpenter-oci/pkg/providers/launchtemplate"
	"karpenter-oci/pkg/providers/securitygroup"
	"karpenter-oci/pkg/providers/subnet"
	"knative.dev/pkg/ptr"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"

	coretest "sigs.k8s.io/karpenter/pkg/test"

	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
	corev1beta1.NormalizedLabels = lo.Assign(corev1beta1.NormalizedLabels)
}

type Environment struct {
	// API
	CmpCli *fake.CmpCli
	VcnCli *fake.VcnCli

	// Cache
	AmiCache                  *cache.Cache
	InstanceTypeCache         *cache.Cache
	SubnetCache               *cache.Cache
	SecurityGroupCache        *cache.Cache
	UnavailableOfferingsCache *ocicache.UnavailableOfferings

	// Providers
	InstanceTypesProvider  *instancetype.Provider
	InstanceProvider       *instance.Provider
	SubnetProvider         *subnet.Provider
	SecurityGroupProvider  *securitygroup.Provider
	AMIProvider            *imagefamily.Provider
	AMIResolver            *imagefamily.Resolver
	LaunchTemplateProvider *launchtemplate.DefaultProvider
}

func NewEnvironment(ctx context.Context, env *coretest.Environment) *Environment {
	// API
	cmpCli := fake.NewCmpCli()
	vcnCli := fake.NewVcnCli()

	// cache
	amiCache := cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval)
	instanceTypeCache := cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval)
	subnetCache := cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval)
	sgCache := cache.New(ocicache.DefaultTTL, ocicache.DefaultCleanupInterval)

	// Providers
	subnetProvider := subnet.NewProvider(vcnCli, subnetCache)
	securityGroupProvider := securitygroup.NewProvider(vcnCli, sgCache)
	amiProvider := imagefamily.NewProvider(cmpCli, amiCache)
	amiResolver := imagefamily.NewResolver(amiProvider)
	unavailableOfferCache := ocicache.NewUnavailableOfferings()
	instanceTypesProvider := instancetype.NewProvider("us-ashburn-1", cmpCli, instanceTypeCache, unavailableOfferCache)
	launchTemplateProvider :=
		launchtemplate.NewDefaultProvider(
			amiResolver,
			ptr.String("ca-bundle"),
			"https://test-cluster",
			"fake_token",
		)
	instanceProvider :=
		instance.NewProvider(
			cmpCli,
			subnetProvider,
			securityGroupProvider,
			launchTemplateProvider,
			unavailableOfferCache,
		)

	return &Environment{
		CmpCli: cmpCli,
		VcnCli: vcnCli,

		AmiCache:                  amiCache,
		InstanceTypeCache:         instanceTypeCache,
		SubnetCache:               subnetCache,
		SecurityGroupCache:        sgCache,
		UnavailableOfferingsCache: unavailableOfferCache,

		InstanceTypesProvider:  instanceTypesProvider,
		InstanceProvider:       instanceProvider,
		SubnetProvider:         subnetProvider,
		SecurityGroupProvider:  securityGroupProvider,
		LaunchTemplateProvider: launchTemplateProvider,
		AMIProvider:            amiProvider,
		AMIResolver:            amiResolver,
	}
}

func (env *Environment) Reset() {
	env.CmpCli.Reset()
	env.VcnCli.Reset()

	env.UnavailableOfferingsCache.Flush()
	env.AmiCache.Flush()
	env.InstanceTypeCache.Flush()
	env.SubnetCache.Flush()

	mfs, err := crmetrics.Registry.Gather()
	if err != nil {
		for _, mf := range mfs {
			for _, metric := range mf.GetMetric() {
				if metric != nil {
					metric.Reset()
				}
			}
		}
	}
}
