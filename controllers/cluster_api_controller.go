package controllers

import (
	"context"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"time"

	clusterregistryv1alpha1 "github.com/minsheng-fintech-corp-ltd/cluster-registry-controller/api/v1alpha1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"
)

var Phase = "Providioneding"

type ClusterApiReconciler struct {
	Client    client.Client
	Log       logr.Logger

	Scheme    *runtime.Scheme
	controller controller.Controller
	// Work queue
	Workqueue workqueue.RateLimitingInterface
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

func (r *ClusterApiReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error){

	ctx := context.Background()
	defer r.Workqueue.ShutDown()

	cluster := &clusterv1.Cluster{}
	secret := &corev1.Secret{}

	log := r.Log.WithValues("Cluster api", req.Namespace)

	if err := r.Client.Get(ctx, req.NamespacedName,cluster); err != nil{
		log.Error(err, "unable fetch Cluster api")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.Workqueue.Add(req.NamespacedName)

	// Process workqueue
	r.ProcessQueue(ctx,cluster,secret)

	return ctrl.Result{}, nil
}

// Create cluster registry
func (r *ClusterApiReconciler) CreateClusterRegistry(ctx context.Context,value client.ObjectKey,cluster *clusterv1.Cluster,config *clientcmdapi.Config){

	log := r.Log.WithValues("Cluster registry",value.Namespace)
	var req ctrl.Request
	req.Name = value.Name+"-cluster-registry"
	req.Namespace = value.Namespace
	clusterreg := &clusterregistryv1alpha1.Cluster{}
	errs := r.Client.Get(ctx,req.NamespacedName,clusterreg)
	if errs != nil{
		log.Info("Create Cluster registry","ClusterRegistry",req.NamespacedName)
		clusterreg := CreateClusterRegistry(req.Name,
			req.Namespace,
			cluster,
			config.Clusters[cluster.Name].CertificateAuthorityData,
			config.Clusters[cluster.Name].Server)
		err := r.Client.Create(ctx,clusterreg)
		if err != nil{
			log.Error(err,"Create Cluster registry fail")
		}
	}else{
		log.Info("Cluster registry already exits")
	}
}

// Get secret according cluster name and namespace
func (r *ClusterApiReconciler) GetSecret(ctx context.Context,value client.ObjectKey,secret *corev1.Secret,cluster *clusterv1.Cluster){
	log := r.Log.WithValues("Secret",value.Namespace)
	var req ctrl.Request
	req.Name = value.Name+"-kubeconfig"
	req.Namespace = value.Namespace
	if err := r.Client.Get(ctx,req.NamespacedName,secret); err != nil{
		log.Info("secret not exit")
		r.Workqueue.Add(value)
	} else {
		fg := secret.Data["value"]
		config,err := clientcmd.Load(fg)
		if err != nil {
			log.Error(err,"Can not load kube-config")
		}
		r.CreateClusterRegistry(ctx,value,cluster,config)
	}
}

// Process Work queue
func (r *ClusterApiReconciler) ProcessQueue(ctx context.Context, cluster *clusterv1.Cluster,secret *corev1.Secret){
	for{
		key, status := r.Workqueue.Get()
		if status{
			r.Log.Info("Cluster queue get element error")
			return
		}
		r.Workqueue.Done(key)

		if value, ok := key.(client.ObjectKey); ok{
				if err := r.Client.Get(ctx, value, cluster); err != nil {
					r.Log.Error(err, "unable fetch Cluster-Api")
				} else {
					status := cluster.Status.Phase
					if status != Phase {
						r.Log.Info("Cluster Api status not ready")
						r.Workqueue.Add(value)
					} else {
						r.Log.Info("Cluster api status ready")
						r.GetSecret(ctx, value, secret, cluster)
					}
				}
		}
		time.Sleep(10 *time.Second)
	}
}

// Setup method for controller
func (r *ClusterApiReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	// cluster-api band with cluster-registry
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Owns(&clusterregistryv1alpha1.Cluster{}).
		WithOptions(options).
		Complete(r)
}

