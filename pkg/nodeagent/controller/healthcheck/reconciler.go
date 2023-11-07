package healthcheck

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// Reconciler checks for containerd and kubelet health and restarts them if required.
type Reconciler struct {
	Hostname                   string
	Client                     client.Client
	Recorder                   record.EventRecorder
	DBus                       dbus.DBus
	HealthCheckers             []HealthChecker
	HealthCheckIntervalSeconds int32
}

// Reconcile executes all defined healtchecks
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconcile Healthchecks")
	node := &metav1.PartialObjectMetadata{}
	node.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))

	if err := r.Client.Get(ctx, request.NamespacedName, node); err != nil {
		return reconcile.Result{}, err
	}

	for _, hc := range r.HealthCheckers {
		hc := hc
		log.Info("Execute", "healthcheck", hc.Name())
		err := hc.Check(ctx)
		if err != nil {
			log.Error(err, "Error during healthcheck", "healthchecker", hc.Name())
		}
	}

	return reconcile.Result{RequeueAfter: time.Duration(r.HealthCheckIntervalSeconds) * time.Second}, nil
}
