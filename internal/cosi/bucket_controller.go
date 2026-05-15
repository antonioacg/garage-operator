/*
Copyright 2026 Raj Singh.

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

package cosi

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cosiv1alpha2 "sigs.k8s.io/container-object-storage-interface/client/apis/objectstorage/v1alpha2"
)

// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=buckets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=buckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=buckets/finalizers,verbs=update

// BucketReconciler reconciles cosiv1alpha2.Bucket objects whose Spec.DriverName
// matches DriverName. It manages the protection finalizer and delegates
// Garage-side bucket lifecycle to Provisioner.
type BucketReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	DriverName  string
	Namespace   string // namespace for shadow GarageBucket resources
	Provisioner *Provisioner
}

func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cosiv1alpha2.Bucket{}).
		WithEventFilter(driverNameMatches(r.DriverName)).
		Named("cosi-bucket").
		Complete(r)
}

func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (reconcile.Result, error) {
	logger := ctrl.LoggerFrom(ctx, "driverName", r.DriverName)
	bucket := &cosiv1alpha2.Bucket{}
	if err := r.Get(ctx, req.NamespacedName, bucket); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if bucket.Spec.DriverName != r.DriverName {
		return reconcile.Result{}, nil
	}

	params, err := ParseBucketClassParameters(bucket.Spec.Parameters, r.Namespace)
	if err != nil {
		return r.fail(ctx, bucket, fmt.Errorf("parse parameters: %w", err))
	}

	if !bucket.GetDeletionTimestamp().IsZero() {
		if bucket.Status.BucketID != "" {
			if err := r.Provisioner.DeleteBucket(ctx, bucket.Status.BucketID, params); err != nil {
				return r.fail(ctx, bucket, err)
			}
		}
		ctrlutil.RemoveFinalizer(bucket, cosiv1alpha2.ProtectionFinalizer)
		if err := r.Update(ctx, bucket); err != nil {
			return reconcile.Result{}, err
		}
		logger.Info("Bucket deleted", "bucketId", bucket.Status.BucketID)
		return reconcile.Result{}, nil
	}

	if bucket.Spec.ExistingBucketID != "" {
		return r.fail(ctx, bucket, errors.New("static provisioning not supported"))
	}

	if ctrlutil.AddFinalizer(bucket, cosiv1alpha2.ProtectionFinalizer) {
		if err := r.Update(ctx, bucket); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	result, err := r.Provisioner.EnsureBucket(ctx, bucket.Name, params)
	if err != nil {
		return r.fail(ctx, bucket, err)
	}

	bucket.Status.ReadyToUse = ptr.To(true)
	bucket.Status.BucketID = result.BucketID
	bucket.Status.Protocols = []cosiv1alpha2.ObjectProtocol{cosiv1alpha2.ObjectProtocolS3}
	bucket.Status.BucketInfo = map[string]string{
		"s3.bucketId":        result.GlobalAlias,
		"s3.endpoint":        result.Endpoint,
		"s3.region":          result.Region,
		"s3.addressingStyle": "Path",
	}
	bucket.Status.Error = nil
	if err := r.Status().Update(ctx, bucket); err != nil {
		return reconcile.Result{}, err
	}
	logger.Info("Bucket ready", "bucketId", result.BucketID)
	return reconcile.Result{}, nil
}

func (r *BucketReconciler) fail(ctx context.Context, bucket *cosiv1alpha2.Bucket, in error) (reconcile.Result, error) {
	bucket.Status.ReadyToUse = ptr.To(false)
	bucket.Status.Error = cosiv1alpha2.NewTimestampedError(time.Now(), in.Error())
	_ = r.Status().Update(ctx, bucket)
	return reconcile.Result{}, in
}
