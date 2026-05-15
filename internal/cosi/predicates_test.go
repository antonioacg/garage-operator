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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cosiv1alpha2 "sigs.k8s.io/container-object-storage-interface/client/apis/objectstorage/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const (
	testGarageDriver = "garage.rajsingh.info"
	testMinioDriver  = "minio.io"
	testDefaultNS    = "default"
)

func TestDriverNameMatches_Bucket(t *testing.T) {
	pred := driverNameMatches(testGarageDriver)

	tests := []struct {
		name   string
		bucket *cosiv1alpha2.Bucket
		want   bool
	}{
		{
			name: "matching driver name",
			bucket: &cosiv1alpha2.Bucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testBucketName,
					Namespace: testDefaultNS,
				},
				Spec: cosiv1alpha2.BucketSpec{
					DriverName: testGarageDriver,
				},
			},
			want: true,
		},
		{
			name: "non-matching driver name",
			bucket: &cosiv1alpha2.Bucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testBucketName,
					Namespace: testDefaultNS,
				},
				Spec: cosiv1alpha2.BucketSpec{
					DriverName: testMinioDriver,
				},
			},
			want: false,
		},
		{
			name: "empty driver name",
			bucket: &cosiv1alpha2.Bucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testBucketName,
					Namespace: testDefaultNS,
				},
				Spec: cosiv1alpha2.BucketSpec{
					DriverName: "",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pred.Create(event.CreateEvent{Object: tt.bucket}))
			assert.Equal(t, tt.want, pred.Update(event.UpdateEvent{ObjectNew: tt.bucket}))
			assert.Equal(t, tt.want, pred.Delete(event.DeleteEvent{Object: tt.bucket}))
			assert.Equal(t, tt.want, pred.Generic(event.GenericEvent{Object: tt.bucket}))
		})
	}
}

func TestDriverNameMatches_BucketAccess(t *testing.T) {
	pred := driverNameMatches(testGarageDriver)

	tests := []struct {
		name   string
		access *cosiv1alpha2.BucketAccess
		want   bool
	}{
		{
			name: "matching driver name",
			access: &cosiv1alpha2.BucketAccess{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAccountName,
					Namespace: testDefaultNS,
				},
				Status: cosiv1alpha2.BucketAccessStatus{
					DriverName: testGarageDriver,
				},
			},
			want: true,
		},
		{
			name: "non-matching driver name",
			access: &cosiv1alpha2.BucketAccess{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAccountName,
					Namespace: testDefaultNS,
				},
				Status: cosiv1alpha2.BucketAccessStatus{
					DriverName: testMinioDriver,
				},
			},
			want: false,
		},
		{
			name: "empty driver name (not set by COSI controller yet)",
			access: &cosiv1alpha2.BucketAccess{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAccountName,
					Namespace: testDefaultNS,
				},
				Status: cosiv1alpha2.BucketAccessStatus{
					DriverName: "",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pred.Create(event.CreateEvent{Object: tt.access}))
			assert.Equal(t, tt.want, pred.Update(event.UpdateEvent{ObjectNew: tt.access}))
			assert.Equal(t, tt.want, pred.Delete(event.DeleteEvent{Object: tt.access}))
			assert.Equal(t, tt.want, pred.Generic(event.GenericEvent{Object: tt.access}))
		})
	}
}

func TestDriverNameMatches_OtherObject(t *testing.T) {
	pred := driverNameMatches(testGarageDriver)

	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBucketName,
			Namespace: testDefaultNS,
		},
	}

	// Objects that don't match Bucket or BucketAccess should always return false
	assert.False(t, pred.Create(event.CreateEvent{Object: configmap}))
	assert.False(t, pred.Update(event.UpdateEvent{ObjectNew: configmap}))
	assert.False(t, pred.Delete(event.DeleteEvent{Object: configmap}))
	assert.False(t, pred.Generic(event.GenericEvent{Object: configmap}))
}
