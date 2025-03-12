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
	"fmt"
	"github.com/imdario/mergo"
	"github.com/samber/lo"
	"karpenter-oci/pkg/operator/options"
)

type OptionsFields struct {
	ClusterName             *string
	ClusterEndpoint         *string
	ClusterCABundle         *string
	BootStrapToken          *string
	SshKey                  *string
	CompartmentId           *string
	VMMemoryOverheadPercent *float64
	FlexCpuMemRatio         *int
	FlexCpuConstrainList    *string
	AvailableDomainPrefix   *string
}

func Options(overrides ...OptionsFields) *options.Options {
	opts := OptionsFields{}
	for _, override := range overrides {
		if err := mergo.Merge(&opts, override, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge settings: %s", err))
		}
	}
	return &options.Options{
		ClusterCABundle:         lo.FromPtrOr(opts.ClusterCABundle, ""),
		ClusterName:             lo.FromPtrOr(opts.ClusterName, "test-cluster"),
		ClusterEndpoint:         lo.FromPtrOr(opts.ClusterEndpoint, "https://test-cluster"),
		BootStrapToken:          lo.FromPtrOr(opts.BootStrapToken, "fake_token"),
		SshKey:                  lo.FromPtrOr(opts.SshKey, "fake_token"),
		CompartmentId:           lo.FromPtrOr(opts.CompartmentId, "fake_compartment_id"),
		VMMemoryOverheadPercent: lo.FromPtrOr(opts.VMMemoryOverheadPercent, 0.075),
		FlexCpuMemRatio:         lo.FromPtrOr(opts.FlexCpuMemRatio, 4),
		FlexCpuConstrainList:    lo.FromPtrOr(opts.FlexCpuConstrainList, "2,4,8,16,32,48,64,96,128"),
		AvailableDomainPrefix:   lo.FromPtrOr(opts.AvailableDomainPrefix, "JPqd"),
	}
}
