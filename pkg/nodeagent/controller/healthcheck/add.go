package healthcheck

import (
	"fmt"
	"net"
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// ControllerName is the name of this controller.
const ControllerName = "healthcheck"

const defaultIntervalSeconds = 30

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName)
	}
	if r.DBus == nil {
		r.DBus = dbus.New()
	}
	if len(r.HealthCheckers) == 0 {
		err := r.setDefaultHealthChecks()
		if err != nil {
			return err
		}
	}
	if r.HealthCheckIntervalSeconds == 0 {
		r.HealthCheckIntervalSeconds = defaultIntervalSeconds
	}

	node := &metav1.PartialObjectMetadata{}
	node.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(node).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

func (r *Reconciler) setDefaultHealthChecks() error {
	clock := clock.RealClock{}

	address := os.Getenv("CONTAINERD_ADDRESS")
	if address == "" {
		address = defaults.DefaultAddress
	}
	namespace := os.Getenv(namespaces.NamespaceEnvVar)
	if namespace == "" {
		namespace = namespaces.Default
	}
	client, err := containerd.New(address, containerd.WithDefaultNamespace(namespace))
	if err != nil {
		return fmt.Errorf("error creating containerd client: %w", err)
	}
	containerdHealthChecker := NewContainerdHealthChecker(r.Client, client, clock, r.Hostname, r.DBus, r.Recorder)

	kubeletHealthChecker := NewKubeletHealthChecker(r.Client, r.Hostname, clock, r.DBus, r.Recorder, net.InterfaceAddrs)
	r.HealthCheckers = []HealthChecker{containerdHealthChecker, kubeletHealthChecker}
	return nil
}
