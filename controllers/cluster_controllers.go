/*
Copyright 2019 The Kubernetes Authors.

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
	_ "encoding/base64"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	//kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	clusterregistry "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Phase = "provisioned"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	Client client.Client
	Log    logr.Logger

	scheme          *runtime.Scheme
	recorder        record.EventRecorder
	controller       controller.Controller
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
func (r *ClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	// Get cluster
	log := r.Log.WithValues("Cluster", req.Namespace)
	log.Info("Get Cluster info")
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName,cluster); err != nil{
		log.Error(err,"unable to fetch Cluster")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	status := cluster.Status.Phase
	if status != Phase{
		err := fmt.Errorf("Cluster phase is: %s",status)
		log.Error(err,"Cluster-api not ready")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get secret from cluster namespace and name
	log_s := r.Log.WithValues("Secret",req.Namespace)
	log_s.Info("Get secret info")
	var req_s ctrl.Request
	req_s.Name = req.Name+"-kubeconfig"
	req_s.Namespace = req.Namespace
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx,req_s.NamespacedName,secret); err != nil{
		log_s.Error(err,"secret not exit")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// load kube config
	fg := secret.Data["value"]
	config,err := clientcmd.Load(fg)
	if err != nil {
		log.Error(err,"Can not load kube-config")
		return ctrl.Result{},nil
	}
	// Find and Create cluster-registry
	log_c := r.Log.WithValues("Cluster-Registry",req.Namespace)
	log_c.Info("Get Cluster-Registry info")
	var req_c ctrl.Request
	req_c.Name = req.Name+"-cluster-registry"
	req_c.Namespace = req.Namespace
	clusterreg := &clusterregistry.Cluster{}
	errs := r.Client.Get(ctx,req_c.NamespacedName,clusterreg)
	if errs != nil{
		log_c.Info("Create Cluster-Registry","ClusterRegistry",req_c.NamespacedName)
		clusterreg := CreateClusterRegistry(req_c.Name,
			req_c.Namespace,
			cluster,
			config.Clusters[cluster.Name].CertificateAuthorityData,
			config.Clusters[cluster.Name].Server)
		err := r.Client.Create(ctx,clusterreg)
		if err != nil{
			log_c.Error(err,"Create Cluster-Registry fail")
		}
	}

	// your logic here
	return ctrl.Result{}, nil
}

func CreateClusterRegistry(name string, namespace string,cluster *clusterv1.Cluster,ca []byte,server string) *clusterregistry.Cluster{
	cr := &clusterregistry.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			Namespace:    namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster.GetObjectMeta(),cluster.GroupVersionKind()),
			},
		},
		Spec: clusterregistry.ClusterSpec{
			KubernetesAPIEndpoints:  clusterregistry.KubernetesAPIEndpoints{
				ServerEndpoints:    []clusterregistry.ServerAddressByClientCIDR{
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

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
    // cluster-api band with cluster-registry
    return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
    	Owns(&clusterregistry.Cluster{}).
		Complete(r)
	return nil
}

