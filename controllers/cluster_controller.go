/*
Copyright 2020 zh.

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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterregistryv1alpha1 "github.com/minsheng-fintech-corp-ltd/cluster-registry-controller/api/v1alpha1"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=clusterregistry.k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clusterregistry.k8s.io,resources=clusters/status,verbs=get;update;patch

func (r *ClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("cluster-registry", req.NamespacedName)
	cluster := &clusterregistryv1alpha1.Cluster{}
	_ = r.Client.Get(ctx, req.NamespacedName,cluster)

	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterregistryv1alpha1.Cluster{}).
		Complete(r)
}

// Create cluster registry resource
func CreateClusterRegistry(name string, namespace string,cluster *clusterv1.Cluster,ca []byte,server string) *clusterregistryv1alpha1.Cluster{
	cr := &clusterregistryv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			Namespace:    namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster.GetObjectMeta(),cluster.GroupVersionKind()),
			},
		},
		Spec: clusterregistryv1alpha1.ClusterSpec{
			KubernetesAPIEndpoints:  clusterregistryv1alpha1.KubernetesAPIEndpoints{
				ServerEndpoints:    []clusterregistryv1alpha1.ServerAddressByClientCIDR{
					{
						ClientCIDR:    "0.0.0.0/0",
						ServerAddress: server,

					},
				},
				CABundle: ca,
			},
		},
	}
	return cr
}
