package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

const (
	// kubeletServiceName is the systemd service name of the kubelet
	kubeletServiceName = "kubelet.service"
	// defaultKubeletHealthEndpoint is the health endpoint of the kubelet
	defaultKubeletHealthEndpoint = "http://127.0.0.1:10248/healthz"
	// maxToggles defines how often the kubelet can chang the readiness during toggleTimeSpan until the node will be hard rebooted
	maxToggles = 5
	// toggleTimeSpan is a floating time window where the kubelet readiness toggles are considered harmful
	toggleTimeSpan = 10 * time.Minute
)

// KubeletHealthChecker configures the kubelet healthcheck
type KubeletHealthChecker struct {
	client       client.Client
	nodeName     string
	firstFailure *time.Time
	Clock        clock.Clock
	dbus         dbus.DBus
	recorder     record.EventRecorder
	// lastInternalIP stores the node internalIP
	lastInternalIP netip.Addr
	getAddresses   func() ([]net.Addr, error)
	// KubeletReadynessToggles contains a entry for every toggle between kubelet Ready->NotReady or NotReady->Ready state
	KubeletReadynessToggles []time.Time
	nodeReady               bool
	kubeletHealthEndpoint   string
}

// NewKubeletHealthChecker create a instance of a kubelet healthcheck
func NewKubeletHealthChecker(client client.Client, nodeName string, clock clock.Clock, dbus dbus.DBus, recorder record.EventRecorder, getAddresses func() ([]net.Addr, error)) *KubeletHealthChecker {
	return &KubeletHealthChecker{
		client:                  client,
		nodeName:                nodeName,
		dbus:                    dbus,
		Clock:                   clock,
		recorder:                recorder,
		getAddresses:            getAddresses,
		KubeletReadynessToggles: []time.Time{},
		kubeletHealthEndpoint:   defaultKubeletHealthEndpoint,
	}
}

// Name returns the name of this healthcheck
func (*KubeletHealthChecker) Name() string {
	return "kubelet"
}

// HasLastInternalIP returns true if the node.InternalIP was stored
// exported for testing
func (k *KubeletHealthChecker) HasLastInternalIP() bool {
	return k.lastInternalIP.IsValid()
}

// SetKubeletHealthEndpoint set the kubeletHealthEndpoint
// exported for testing
func (k *KubeletHealthChecker) SetKubeletHealthEndpoint(kubeletHealthEndpoint string) {
	k.kubeletHealthEndpoint = kubeletHealthEndpoint
}

// Check performs the actual health check for the kubelet
func (k *KubeletHealthChecker) Check(ctx context.Context) error {
	log := logf.FromContext(ctx).WithName("kubelet")
	node := &corev1.Node{}
	err := k.client.Get(ctx, types.NamespacedName{Name: k.nodeName}, node)
	if err != nil {
		return fmt.Errorf("unable to fetch node %w", err)
	}
	// log.Info("got node", "node", fmt.Sprintf("%#v", node))

	// This mimics the old behavior of the kubelet-health-monitor which checks for
	// to many Ready->NoReady toggles in a certain time and reboots the node in such a case.
	if k.nodeReady != isNodeReady(node) {
		needsReboot := k.ToggleKubeletState()
		if needsReboot {
			log.Info("Kubelet toggled between Ready and NotReady too often, reboot the node now")
			k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "toggled between Ready and NotReady more than %d times in a %s time window, reboot the node now to resolve this", maxToggles, toggleTimeSpan)
			err = k.dbus.Reboot()
			if err != nil {
				return fmt.Errorf("unable to reboot node %w", err)
			}
		}
	}
	k.nodeReady = isNodeReady(node)

	err = k.ensureNodeInternalIP(ctx, node)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, k.kubeletHealthEndpoint, nil)
	if err != nil {
		log.Error(err, "Unable to fetch kubelet health")
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		log.Error(err, "Unable to fetch kubelet health with http get request")
	}
	if err == nil && response.StatusCode == http.StatusOK {
		if k.firstFailure != nil {
			log.Info("Kubelet is healthy again")
			k.recorder.Event(node, corev1.EventTypeNormal, "kubelet", "healthy")
			k.firstFailure = nil
		}
		log.Info("Kubelet is healthy again", "statusCode", response.StatusCode)
		return nil
	}
	if k.firstFailure == nil {
		now := k.Clock.Now()
		k.firstFailure = &now
		log.Error(err, "Kubelet is not healthy")
		k.recorder.Event(node, corev1.EventTypeWarning, "kubelet", "unhealthy")
	}

	if time.Since(*k.firstFailure).Abs() < maxFailureDuration {
		return nil
	}

	log.Error(err, "Kubelet is not healthy, restarting")
	err = k.dbus.Restart(ctx, k.recorder, node, kubeletServiceName)
	if err == nil {
		k.firstFailure = nil
	}
	return err
}

// ensureNodeInternalIP mimics the old weird logic which restores the internalIP of the node
// if this was lost for some reason.
func (k *KubeletHealthChecker) ensureNodeInternalIP(ctx context.Context, node *corev1.Node) error {
	log := logf.FromContext(ctx).WithName("kubelet")
	var (
		externalIP string
		internalIP string
	)
	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case corev1.NodeExternalIP:
			externalIP = addr.Address
		case corev1.NodeInternalIP:
			internalIP = addr.Address
		default:
			// ignore
		}
	}

	if externalIP == "" && internalIP == "" {
		addresses, err := k.getAddresses()
		if err != nil {
			return fmt.Errorf("unable to list all network interface IP addresses")
		}

		for _, addr := range addresses {
			parsed, err := netip.ParsePrefix(addr.String())
			parsedIP := parsed.Addr()
			if err != nil {
				return fmt.Errorf("unable to parse IP address %w", err)
			}
			if parsedIP.Compare(k.lastInternalIP) != 0 {
				continue
			}

			// one of the ip addresses on the node matches the previous set internalIP of the node which is no gone, set it again.
			node.Status.Addresses = []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: k.lastInternalIP.String(),
				},
			}

			err = k.client.Status().Update(ctx, node)
			if err != nil {
				log.Error(err, "Unable to update node with internal IP")
				k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "unable to update node with internal IP: %s", err.Error())
				return k.dbus.Restart(ctx, k.recorder, node, kubeletServiceName)
			}
			log.Info("Updated internal IP address of node", "ip", k.lastInternalIP.String())
			k.recorder.Eventf(node, corev1.EventTypeNormal, "kubelet", "updated the lost internal IP address of node to the previous known: %s ", k.lastInternalIP.String())
		}
	} else {
		var err error
		k.lastInternalIP, err = netip.ParseAddr(internalIP)
		if err != nil {
			return fmt.Errorf("unable to parse internal IP address %w", err)
		}
	}

	return nil
}

// ToggleKubeletState should be triggered if the state of the kubelet changed from Ready -> NotReady or vise versa.
// it returns true if a reboot of the node should be triggered
func (k *KubeletHealthChecker) ToggleKubeletState() bool {
	k.KubeletReadynessToggles = append(k.KubeletReadynessToggles, time.Now())
	if len(k.KubeletReadynessToggles) < maxToggles {
		return false
	}

	// We have more than maxToggles reached but in a too long period
	if k.Clock.Since(k.KubeletReadynessToggles[0]).Abs() > toggleTimeSpan.Abs() {
		k.KubeletReadynessToggles = k.KubeletReadynessToggles[:len(k.KubeletReadynessToggles)-1]
		return false
	}

	// if the oldest entry is older than allowed, trim the slices.
	if len(k.KubeletReadynessToggles) > maxToggles {
		k.KubeletReadynessToggles = k.KubeletReadynessToggles[:len(k.KubeletReadynessToggles)-1]
	}

	// reaching this point means we need to reboot
	clear(k.KubeletReadynessToggles)
	return true
}

// isNodeReady returns true if a node is ready; false otherwise.
func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
