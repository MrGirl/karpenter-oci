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

package garbagecollection

import (
	"context"
	"fmt"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"github.com/zoom/karpenter-oci/pkg/apis/v1alpha1"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	v1core "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"time"
)

type Controller struct {
	kubeClient      client.Client
	cloudProvider   cloudprovider.CloudProvider
	successfulCount uint64 // keeps track of successful reconciles for more aggressive requeueing near the start of the controller
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:      kubeClient,
		cloudProvider:   cloudProvider,
		successfulCount: 0,
	}
}

func (c *Controller) Name() string {
	return "instance.garbagecollection"
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	// We LIST machines on the CloudProvider BEFORE we grab Machines/Nodes on the cluster so that we make sure that, if
	// LISTing instances takes a long time, our information is more updated by the time we get to Machine and Node LIST
	// This works since our CloudProvider instances are deleted based on whether the Machine exists or not, not vise-versa
	retrieved, err := c.cloudProvider.List(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing cloudprovider machines, %w", err)
	}
	managedRetrieved := lo.Filter(retrieved, func(nc *v1core.NodeClaim, _ int) bool {
		return nc.Annotations[v1alpha1.ManagedByAnnotationKey] != "" && nc.DeletionTimestamp.IsZero()
	})
	terminated := lo.Filter(retrieved, func(nc *v1core.NodeClaim, _ int) bool {
		return nc.Annotations[v1alpha1.ManagedByAnnotationKey] != "" && !nc.DeletionTimestamp.IsZero()
	})
	nodeClaimList := &v1core.NodeClaimList{}
	if err = c.kubeClient.List(ctx, nodeClaimList); err != nil {
		return reconcile.Result{}, err
	}
	nodeList := &v1.NodeList{}
	if err = c.kubeClient.List(ctx, nodeList); err != nil {
		return reconcile.Result{}, err
	}
	resolvedProviderIDs := sets.New[string](lo.FilterMap(nodeClaimList.Items, func(n v1core.NodeClaim, _ int) (string, bool) {
		return n.Status.ProviderID, n.Status.ProviderID != ""
	})...)
	errs := make([]error, len(retrieved))
	workqueue.ParallelizeUntil(ctx, 10, len(managedRetrieved), func(i int) {
		if !resolvedProviderIDs.Has(managedRetrieved[i].Status.ProviderID) &&
			time.Since(managedRetrieved[i].CreationTimestamp.Time) > time.Second*30 {
			errs[i] = c.garbageCollect(ctx, managedRetrieved[i], nodeList)
		}
	})
	errs2 := make([]error, len(retrieved))
	workqueue.ParallelizeUntil(ctx, 10, len(terminated), func(i int) {
		if !resolvedProviderIDs.Has(terminated[i].Status.ProviderID) &&
			time.Since(terminated[i].CreationTimestamp.Time) > time.Minute*5 {
			errs2[i] = c.garbageNode(ctx, terminated[i], nodeList)
		}
	})
	if err = multierr.Combine(append(errs, errs2...)...); err != nil {
		return reconcile.Result{}, err
	}
	c.successfulCount++
	return reconcile.Result{RequeueAfter: lo.Ternary(c.successfulCount <= 20, time.Second*10, time.Minute*2)}, nil
}

func (c *Controller) garbageCollect(ctx context.Context, nodeClaim *v1core.NodeClaim, nodeList *v1.NodeList) error {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("provider-id", nodeClaim.Status.ProviderID))
	if err := c.cloudProvider.Delete(ctx, nodeClaim); cloudprovider.IgnoreNodeClaimNotFoundError(err) != nil {
		return err
	}
	log.FromContext(ctx).V(1).Info("garbage collected cloudprovider instance")

	// Go ahead and cleanup the node if we know that it exists to make scheduling go quicker
	if node, ok := lo.Find(nodeList.Items, func(n v1.Node) bool {
		return n.Spec.ProviderID == nodeClaim.Status.ProviderID
	}); ok {
		if err := c.kubeClient.Delete(ctx, &node); err != nil {
			return client.IgnoreNotFound(err)
		}
		log.FromContext(ctx).WithValues("Node", klog.KRef("", node.Name)).V(1).Info("garbage collected node")
	}
	return nil
}

func (c *Controller) garbageNode(ctx context.Context, nodeClaim *v1core.NodeClaim, nodeList *v1.NodeList) error {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("provider-id", nodeClaim.Status.ProviderID))

	// Go ahead and cleanup the node if we know that it exists to make scheduling go quicker
	if node, ok := lo.Find(nodeList.Items, func(n v1.Node) bool {
		return n.Spec.ProviderID == nodeClaim.Status.ProviderID
	}); ok {
		// double check if node is unreachable
		if _, unreachable := lo.Find(node.Spec.Taints, func(item v1.Taint) bool {
			return item.Key == v1.TaintNodeUnreachable
		}); unreachable {
			if err := c.kubeClient.Delete(ctx, &node); err != nil {
				return client.IgnoreNotFound(err)
			}
			log.FromContext(ctx).WithValues("Node", klog.KRef("", node.Name)).V(1).Info("garbage collected node")
		}
	}
	return nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instance.garbagecollection").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
