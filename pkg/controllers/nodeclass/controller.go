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

package nodeclass

import (
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"karpenter-oci/pkg/apis/v1alpha1"
	"karpenter-oci/pkg/providers/imagefamily"
	"karpenter-oci/pkg/providers/subnet"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/events"
	corecontroller "sigs.k8s.io/karpenter/pkg/operator/controller"
	"time"
)

var _ corecontroller.FinalizingTypedController[*v1alpha1.OciNodeClass] = (*Controller)(nil)

type Controller struct {
	kubeClient          client.Client
	recorder            events.Recorder
	subnetProvider      *subnet.Provider
	imageFamilyProvider *imagefamily.Provider
}

func NewController(kubeClient client.Client, recorder events.Recorder, imageProvider *imagefamily.Provider, subnetProvider *subnet.Provider) corecontroller.Controller {

	return corecontroller.Typed[*v1alpha1.OciNodeClass](kubeClient, &Controller{
		kubeClient:          kubeClient,
		recorder:            recorder,
		subnetProvider:      subnetProvider,
		imageFamilyProvider: imageProvider,
	})
}

func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1alpha1.OciNodeClass) (reconcile.Result, error) {
	stored := nodeClass.DeepCopy()
	controllerutil.AddFinalizer(nodeClass, v1beta1.TerminationFinalizer)

	if nodeClass.Annotations[v1alpha1.AnnotationOciNodeClassHashVersion] != v1alpha1.OciNodeClassHashVersion {
		if err := c.updateNodeClaimHash(ctx, nodeClass); err != nil {
			return reconcile.Result{}, err
		}
	}
	nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
		v1alpha1.AnnotationOciNodeClassHash:        nodeClass.Hash(),
		v1alpha1.AnnotationOciNodeClassHashVersion: v1alpha1.OciNodeClassHashVersion,
	})

	err := multierr.Combine(
		c.resolveSubnets(ctx, nodeClass),
		c.resolveAMIs(ctx, nodeClass),
	)
	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		statusCopy := nodeClass.DeepCopy()
		if patchErr := c.kubeClient.Patch(ctx, nodeClass, client.MergeFrom(stored)); err != nil {
			err = multierr.Append(err, client.IgnoreNotFound(patchErr))
		}
		if patchErr := c.kubeClient.Status().Patch(ctx, statusCopy, client.MergeFrom(stored)); err != nil {
			err = multierr.Append(err, client.IgnoreNotFound(patchErr))
		}
	}
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Finalize(ctx context.Context, nodeClass *v1alpha1.OciNodeClass) (reconcile.Result, error) {
	stored := nodeClass.DeepCopy()
	if !controllerutil.ContainsFinalizer(nodeClass, v1beta1.TerminationFinalizer) {
		return reconcile.Result{}, nil
	}
	nodeClaimList := &corev1beta1.NodeClaimList{}
	if err := c.kubeClient.List(ctx, nodeClaimList, client.MatchingFields{"spec.nodeClassRef.name": nodeClass.Name}); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing nodeclaims that are using nodeclass, %w", err)
	}
	if len(nodeClaimList.Items) > 0 {
		c.recorder.Publish(WaitingOnNodeClaimTerminationEvent(nodeClass, lo.Map(nodeClaimList.Items, func(nc corev1beta1.NodeClaim, _ int) string { return nc.Name })))
		return reconcile.Result{RequeueAfter: time.Minute * 10}, nil // periodically fire the event
	}
	controllerutil.RemoveFinalizer(nodeClass, v1beta1.TerminationFinalizer)
	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, client.IgnoreNotFound(fmt.Errorf("removing termination finalizer, %w", err))
		}
	}
	return reconcile.Result{}, nil
}

func (c *Controller) resolveSubnets(ctx context.Context, nodeClass *v1alpha1.OciNodeClass) error {
	subnets, err := c.subnetProvider.List(ctx, nodeClass)
	if err != nil {
		return err
	}
	if len(subnets) == 0 {
		nodeClass.Status.Subnets = nil
		return fmt.Errorf("no subnets exist given constraints %v", nodeClass.Spec.SubnetName)
	}
	nodeClass.Status.Subnets = lo.Map(subnets, func(ociSubnet core.Subnet, _ int) *v1alpha1.Subnet {
		return &v1alpha1.Subnet{
			Id:   *ociSubnet.Id,
			Name: *ociSubnet.DisplayName,
		}
	})
	return nil
}

func (c *Controller) resolveAMIs(ctx context.Context, nodeClass *v1alpha1.OciNodeClass) error {
	images, err := c.imageFamilyProvider.List(ctx, nodeClass)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		nodeClass.Status.Image = nil
		return fmt.Errorf("no amis exist given constraints")
	}
	nodeClass.Status.Image = lo.Map(images, func(image core.Image, _ int) *v1alpha1.Image {
		return &v1alpha1.Image{
			Name: *image.DisplayName,
			Id:   *image.Id,
		}
	})[0]

	return nil
}

// Updating `ec2nodeclass-hash-version` annotation inside the karpenter controller means a breaking change has been made to the hash calculation.
// `ec2nodeclass-hash` annotation on the EC2NodeClass will be updated, due to the breaking change, making the `ec2nodeclass-hash` on the NodeClaim different from
// EC2NodeClass. Since, we cannot rely on the `ec2nodeclass-hash` on the NodeClaims, due to the breaking change, we will need to re-calculate the hash and update the annotation.
// For more information on the Drift Hash Versioning: https://github.com/kubernetes-sigs/karpenter/blob/main/designs/drift-hash-versioning.md
func (c *Controller) updateNodeClaimHash(ctx context.Context, nodeClass *v1alpha1.OciNodeClass) error {
	ncList := &corev1beta1.NodeClaimList{}
	if err := c.kubeClient.List(ctx, ncList, client.MatchingFields{"spec.nodeClassRef.name": nodeClass.Name}); err != nil {
		return err
	}

	errs := make([]error, len(ncList.Items))
	for i := range ncList.Items {
		nc := ncList.Items[i]
		stored := nc.DeepCopy()

		if nc.Annotations[v1alpha1.AnnotationOciNodeClassHashVersion] != v1alpha1.OciNodeClassHashVersion {
			nc.Annotations = lo.Assign(nc.Annotations, map[string]string{
				v1alpha1.AnnotationOciNodeClassHashVersion: v1alpha1.OciNodeClassHashVersion,
			})

			// Any NodeClaim that is already drifted will remain drifted if the karpenter.k8s.aws/nodepool-hash-version doesn't match
			// Since the hashing mechanism has changed we will not be able to determine if the drifted status of the NodeClaim has changed
			if nc.StatusConditions().GetCondition(corev1beta1.Drifted) == nil {
				nc.Annotations = lo.Assign(nc.Annotations, map[string]string{
					v1alpha1.AnnotationOciNodeClassHash: nodeClass.Hash(),
				})
			}

			if !equality.Semantic.DeepEqual(stored, nc) {
				if err := c.kubeClient.Patch(ctx, &nc, client.MergeFrom(stored)); err != nil {
					errs[i] = client.IgnoreNotFound(err)
				}
			}
		}
	}

	return multierr.Combine(errs...)
}

func (c *Controller) Name() string {
	return "nodeclass"
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1alpha1.OciNodeClass{}).
		Watches(
			&corev1beta1.NodeClaim{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
				nc := o.(*corev1beta1.NodeClaim)
				if nc.Spec.NodeClassRef == nil {
					return nil
				}
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: nc.Spec.NodeClassRef.Name}}}
			}),
			// Watch for NodeClaim deletion events
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool { return false },
				UpdateFunc: func(e event.UpdateEvent) bool { return false },
				DeleteFunc: func(e event.DeleteEvent) bool { return true },
			}),
		).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(100*time.Millisecond, 1*time.Minute),
				// 10 qps, 100 bucket size
				&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
			MaxConcurrentReconciles: 10,
		}))
}
