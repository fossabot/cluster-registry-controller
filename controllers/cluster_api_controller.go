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

var Phase = "Providioned"

type ClusterApiReconciler struct {
	Client    client.Client
	Log       logr.Logger

	Scheme    *runtime.Scheme
	controller controller.Controller
	// Work queue
	Workqueue workqueue.RateLimitingInterface

	// Log interval time
	Interval   time.Duration
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

func (r *ClusterApiReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error){

	ctx := context.Background()

	// defer r.Workqueue.ShutDown()

	cluster := &clusterv1.Cluster{}
	secret := &corev1.Secret{}

	log := r.Log.WithValues("Cluster api", req.Namespace)

	if err := r.Client.Get(ctx, req.NamespacedName,cluster); err != nil{
		log.Error(err, "unable fetch Cluster api")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	//go r.reconcilStatus(ctx,req,cluster,secret)
	//go r.reconcilSecret(ctx,req,secret,cluster)

	log.Info("add queue")
	r.Workqueue.Add(req.NamespacedName)
	value, ok := r.Workqueue.Get()
	r.ProcessQueue(ctx,cluster,secret,value,ok)
	//for {
	//	fmt.Printf("queue len; %d\n",r.Workqueue.Len())
	//	if !ok {
	//		key, status := value.(client.ObjectKey)
	//		if status {
	//			log.Info(key.String())
	//		}
	//	}
	//	if err := r.Client.Get(ctx, req.NamespacedName,cluster); err != nil{
	//		log.Error(err, "unable fetch Cluster api")
	//		r.Workqueue.Done(value)
	//		return ctrl.Result{}, client.IgnoreNotFound(err)
	//	}
	//	time.Sleep(5*time.Second)
	//}

	// Process workqueue
	//log.Info("Add workqueue")
	//r.ProcessQueue(ctx,cluster,secret)

	return ctrl.Result{}, nil
}

// Create cluster registry
func (r *ClusterApiReconciler) CreateClusterRegistry(ctx context.Context,value client.ObjectKey,cluster *clusterv1.Cluster,config *clientcmdapi.Config) error {

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
			return err
		}
	}else{
		log.Info("Cluster registry already exits")
		return nil
	}
	return nil
}

// Get secret according cluster name and namespace
func (r *ClusterApiReconciler) GetSecret(ctx context.Context,value client.ObjectKey,secret *corev1.Secret,cluster *clusterv1.Cluster) error {
	log := r.Log.WithValues("Secret namespace",value.Namespace)
	var req ctrl.Request
	req.Name = value.Name+"-kubeconfig"
	req.Namespace = value.Namespace
	if err := r.Client.Get(ctx,req.NamespacedName,secret); err != nil{
		log.Info("secret not exit","secret",req.NamespacedName)
		return err
	} else {
		fg := secret.Data["value"]
		config,err := clientcmd.Load(fg)
		if err != nil {
			log.Error(err,"Can not load kube-config")
		}
		return r.CreateClusterRegistry(ctx,value,cluster,config)
	}
	return nil
}

// Process Work queue
func (r *ClusterApiReconciler) ProcessQueue(ctx context.Context, cluster *clusterv1.Cluster,secret *corev1.Secret,key interface{},status bool) (ctrl.Result, error){
	log := r.Log.WithValues("namespace",cluster.Namespace)
	for{
		if status{
			log.Info("Cluster queue get element error")
			return ctrl.Result{}, nil
		}

		if value, ok := key.(client.ObjectKey); ok {
			err := r.Client.Get(ctx,value,cluster)
			if err != nil{
				log.Error(err, "unable fetch cluster api")
				r.Workqueue.Done(value)
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			status := cluster.Status.Phase
			if status != Phase {
				log.Info("Cluster api status not ready","cluster name:",cluster.Name)
			} else {
				log.Info("Cluster api status ready","cluster name:",cluster.Name)
				err := r.GetSecret(ctx, value, secret, cluster)
				if err == nil{
					r.Workqueue.Done(value)
					return ctrl.Result{},nil
				}
			}
		}
		time.Sleep(r.Interval *time.Second)
		}
	return ctrl.Result{}, nil
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

