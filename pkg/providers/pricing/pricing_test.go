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

package pricing_test

import (
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/example/helpers"
	"github.com/oracle/oci-go-sdk/v65/onesubscription"
	"testing"
)

func TestPricing(t *testing.T) {
	ExampleListRateCards()
}

func ExampleListRateCards() {
	// Create a default authentication provider that uses the DEFAULT
	// profile in the configuration file.
	// Refer to <see href="https://docs.cloud.oracle.com/en-us/iaas/Content/API/Concepts/sdkconfig.htm#SDK_and_CLI_Configuration_File>the public documentation</see> on how to prepare a configuration file.
	client, err := onesubscription.NewRatecardClientWithConfigurationProvider(common.CustomProfileSessionTokenConfigProvider("~/.oci/config", "SESSION"))
	helpers.FatalIfError(err)

	// Create a request and dependent object(s).

	req := onesubscription.ListRateCardsRequest{
		SubscriptionId: common.String("SubscriptionId"),
		//TimeFrom:      &common.SDKTime{Time: time.Now()},
		//Limit:         common.Int(238),
		//OpcRequestId:  common.String("KYG12NUX61NDN0U4CRQB<unique_ID>"),
		//Page:          common.String("EXAMPLE-page-Value"),
		//SortOrder:     onesubscription.ListRateCardsSortOrderDesc,
		CompartmentId: common.String("CompartmentId"),
		//PartNumber:    common.String("EXAMPLE-partNumber-Value"),
		//SortBy:        onesubscription.ListRateCardsSortByTimeinvoicing,
		//TimeTo:        &common.SDKTime{Time: time.Now()}
	}

	// Send the request using the service client
	resp, err := client.ListRateCards(context.Background(), req)
	helpers.FatalIfError(err)

	// Retrieve value from the response.
	fmt.Println(resp)
}
