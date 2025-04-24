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

package operator

import (
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/samber/lo"
	"karpenter-oci/pkg/utils"
	"os/user"
	"testing"
)

func TestAD(t *testing.T) {
	configProvider := common.CustomProfileSessionTokenConfigProvider(fmt.Sprintf("%s/.oci/config", lo.Must(user.Current()).HomeDir), "SESSION")
	client := lo.Must(identity.NewIdentityClientWithConfigurationProvider(configProvider))
	req := identity.ListAvailabilityDomainsRequest{CompartmentId: common.String("compartmentId")}
	resp := lo.Must(client.ListAvailabilityDomains(context.Background(), req))
	configProvider.Region()
	for _, item := range resp.Items {
		fmt.Printf("%s %s\n", utils.ToString(item.Name), utils.ToString(item.Id))
	}
}
