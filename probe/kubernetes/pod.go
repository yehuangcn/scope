package kubernetes

import (
	"strings"
	"time"

	"github.com/weaveworks/scope/report"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
)

// These constants are keys used in node metadata
const (
	PodID          = "kubernetes_pod_id"
	PodName        = "kubernetes_pod_name"
	PodCreated     = "kubernetes_pod_created"
	PodState       = "kubernetes_pod_state"
	PodLabelPrefix = "kubernetes_pod_labels_"
	PodIP          = "kubernetes_pod_ip"
	ServiceIDs     = "kubernetes_service_ids"

	StateDeleted = "deleted"
)

// Pod represents a Kubernetes pod
type Pod interface {
	UID() string
	ID() string
	Name() string
	Namespace() string
	Created() string
	AddServiceID(id string)
	Labels() labels.Labels
	NodeName() string
	GetNode(probeID string) report.Node
}

type pod struct {
	*api.Pod
	serviceIDs report.StringSet
	Node       *api.Node
}

// NewPod creates a new Pod
func NewPod(p *api.Pod) Pod {
	return &pod{Pod: p, serviceIDs: report.MakeStringSet()}
}

func (p *pod) UID() string {
	// Work around for master pod not reporting the right UID.
	if hash, ok := p.ObjectMeta.Annotations["kubernetes.io/config.hash"]; ok {
		return hash
	}
	return string(p.ObjectMeta.UID)
}

func (p *pod) ID() string {
	return p.ObjectMeta.Namespace + "/" + p.ObjectMeta.Name
}

func (p *pod) Name() string {
	return p.ObjectMeta.Name
}

func (p *pod) Namespace() string {
	return p.ObjectMeta.Namespace
}

func (p *pod) Created() string {
	return p.ObjectMeta.CreationTimestamp.Format(time.RFC822)
}

func (p *pod) Labels() labels.Labels {
	return labels.Set(p.ObjectMeta.Labels)
}

func (p *pod) AddServiceID(id string) {
	p.serviceIDs = p.serviceIDs.Add(id)
}

func (p *pod) State() string {
	return string(p.Status.Phase)
}

func (p *pod) NodeName() string {
	return p.Spec.NodeName
}

func (p *pod) GetNode(probeID string) report.Node {
	n := report.MakeNodeWith(report.MakePodNodeID(p.UID()), map[string]string{
		PodID:      p.ID(),
		PodName:    p.Name(),
		Namespace:  p.Namespace(),
		PodCreated: p.Created(),
		PodState:   p.State(),
		PodIP:      p.Status.PodIP,
		report.ControlProbeID: probeID,
	}).WithSets(report.EmptySets.Add(ServiceIDs, p.serviceIDs))
	for _, serviceID := range p.serviceIDs {
		segments := strings.SplitN(serviceID, "/", 2)
		if len(segments) != 2 {
			continue
		}
		n = n.WithParents(report.EmptySets.
			Add(report.Service, report.MakeStringSet(report.MakeServiceNodeID(p.Namespace(), segments[1]))),
		)
	}
	n = n.AddTable(PodLabelPrefix, p.ObjectMeta.Labels)
	n = n.WithControls(GetLogs, DeletePod)
	return n
}
