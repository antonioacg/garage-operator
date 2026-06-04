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

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	garagev1beta1 "github.com/rajsinghtech/garage-operator/api/v1beta1"
	garagev1beta2 "github.com/rajsinghtech/garage-operator/api/v1beta2"
)

// Cluster-delete must not layout-evict user-owned (Manual) GarageNodes. They
// have no controllerRef to the cluster (K8s GC never deletes them) and own their
// own layout lifecycle, so the cluster finalizer's collectGarageNodeIDs must
// only gather operator-managed node IDs.
var _ = Describe("collectGarageNodeIDs excludes user-owned nodes", func() {
	const (
		clusterName = "finalize-skip-cluster"
		opNodeID    = "1111111111111111111111111111111111111111111111111111111111111111"
		userNodeID  = "2222222222222222222222222222222222222222222222222222222222222222"
	)
	cleanup := func(name string, node bool) {
		if node {
			n := &garagev1beta1.GarageNode{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: testNamespace}, n); err == nil {
				n.Finalizers = nil
				_ = k8sClient.Update(ctx, n)
				_ = k8sClient.Delete(ctx, n)
			}
			return
		}
		c := &garagev1beta2.GarageCluster{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: testNamespace}, c); err == nil {
			c.Finalizers = nil
			_ = k8sClient.Update(ctx, c)
			_ = k8sClient.Delete(ctx, c)
		}
	}

	AfterEach(func() {
		cleanup("op-owned", true)
		cleanup("user-owned", true)
		cleanup(clusterName, false)
	})

	It("collects operator-managed node IDs but skips user-owned ones", func() {
		r := &GarageClusterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		cluster := &garagev1beta2.GarageCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: testNamespace},
			Spec: garagev1beta2.GarageClusterSpec{
				LayoutPolicy: LayoutPolicyManual,
				Storage: &garagev1beta2.StorageSpec{
					Replicas: 1,
					Metadata: &garagev1beta2.VolumeConfig{Size: ptrQuantity(resource.MustParse("1Gi"))},
					Data:     &garagev1beta2.VolumeConfig{Size: ptrQuantity(resource.MustParse("10Gi"))},
				},
				Replication: &garagev1beta2.ReplicationConfig{Factor: 1},
			},
		}
		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

		mkNode := func(name, nodeID string, operatorManaged bool) *garagev1beta1.GarageNode {
			labels := map[string]string{}
			if operatorManaged {
				labels[labelAppManagedBy] = managedByOperatorValue
			}
			cap := resource.MustParse("10Gi")
			return &garagev1beta1.GarageNode{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace, Labels: labels},
				Spec: garagev1beta1.GarageNodeSpec{
					ClusterRef: garagev1beta1.ClusterReference{Name: clusterName},
					Zone:       testNodeZone,
					Capacity:   &cap,
					NodeID:     nodeID,
					Storage:    &garagev1beta1.NodeStorageConfig{Data: &garagev1beta1.NodeVolumeConfig{Size: ptrQuantity(resource.MustParse("10Gi"))}},
				},
			}
		}
		Expect(k8sClient.Create(ctx, mkNode("op-owned", opNodeID, true))).To(Succeed())
		Expect(k8sClient.Create(ctx, mkNode("user-owned", userNodeID, false))).To(Succeed())

		// envtest list is cache-backed via the same client; create is synchronous to the API.
		Eventually(func(g Gomega) {
			got := r.collectGarageNodeIDs(ctx, cluster)
			g.Expect(got).To(HaveKey(opNodeID), "operator-managed node must be collected for layout eviction")
			g.Expect(got).NotTo(HaveKey(userNodeID), "user-owned node must NOT be layout-evicted on cluster delete")
		}).Should(Succeed())

		// Sanity: deleting the cluster CR must not have been required; the guard is
		// purely ownership-label based.
		fetched := &garagev1beta1.GarageNode{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "user-owned", Namespace: testNamespace}, fetched)).To(Succeed())
		Expect(fetched.Labels).NotTo(HaveKey(labelAppManagedBy))
	})
})
