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
	cosiv1alpha2 "sigs.k8s.io/container-object-storage-interface/client/apis/objectstorage/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// driverNameMatches returns a predicate that admits objects whose COSI driver
// name matches the supplied value. Works on both Bucket.Spec.DriverName and
// BucketAccess.Status.DriverName (the latter is populated by the cluster-wide
// COSI controller, not by callers).
func driverNameMatches(driverName string) predicate.Predicate {
	match := func(obj any) bool {
		switch v := obj.(type) {
		case *cosiv1alpha2.Bucket:
			return v.Spec.DriverName == driverName
		case *cosiv1alpha2.BucketAccess:
			return v.Status.DriverName == driverName
		}
		return false
	}
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return match(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return match(e.ObjectNew) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return match(e.Object) },
		GenericFunc: func(e event.GenericEvent) bool { return match(e.Object) },
	}
}
