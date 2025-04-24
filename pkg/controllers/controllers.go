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

package controllers

import (
	"context"
	"karpenter-oci/pkg/controllers/nodeclaim/garbagecollection"
	"karpenter-oci/pkg/controllers/nodeclass"
	"karpenter-oci/pkg/providers/imagefamily"
	"karpenter-oci/pkg/providers/subnet"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/controller"
)

func NewControllers(ctx context.Context, kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, recorder events.Recorder, imageProvider *imagefamily.Provider, subnetProvider *subnet.Provider) []controller.Controller {
	controllers := []controller.Controller{
		nodeclass.NewController(kubeClient, recorder, imageProvider, subnetProvider),
		garbagecollection.NewController(kubeClient, cloudProvider)}
	return controllers
}
