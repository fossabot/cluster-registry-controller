module kubebuilder-without-api

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/pkg/errors v0.8.1
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/cluster-registry v0.0.6
	sigs.k8s.io/cluster-api v0.2.10
	sigs.k8s.io/controller-runtime v0.5.0
)
