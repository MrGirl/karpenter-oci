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

package cache

import "time"

const (
	DefaultTTL = time.Minute
	// InstanceTypesAndZonesTTL is the time before we refresh instance types and zones at Oci instance
	InstanceTypesAndZonesTTL = 5 * time.Minute

	// UnavailableOfferingsTTL is the time before offerings that were marked as unavailable
	// are removed from the cache and are available for launch again
	UnavailableOfferingsTTL = 3 * time.Minute
)

const (
	// DefaultCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	DefaultCleanupInterval = time.Minute

	// UnavailableOfferingsCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	// We drop the cleanup interval down for the ICE cache to get quicker reactivity to offerings
	// that become available after they get evicted from the cache
	UnavailableOfferingsCleanupInterval = time.Second * 10
)
