package testenv

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newReadyNode(name, zone string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topologyZoneLabel: zone,
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func newTestDeploymentWithNodes(t *testing.T, nodes ...*corev1.Node) *Deployment {
	t.Helper()

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	objects := make([]runtime.Object, 0, len(nodes))
	for _, node := range nodes {
		objects = append(objects, node)
	}

	client := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objects...).Build()

	return &Deployment{
		testenv: &TestCaseEnv{
			kubeClient: client,
			Log:        logr.Discard(),
		},
	}
}

func TestMultisiteSiteNames(t *testing.T) {
	got := multisiteSiteNames(3)
	want := []string{"site1", "site2", "site3"}

	if len(got) != len(want) {
		t.Fatalf("unexpected site count: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected site at index %d: got=%s want=%s", i, got[i], want[i])
		}
	}
}

func TestMultisiteManagerDefaultsUsesSiteCount(t *testing.T) {
	defaults := multisiteManagerDefaults(4)
	if !strings.Contains(defaults, "all_sites: site1,site2,site3,site4") {
		t.Fatalf("defaults missing expected all_sites line: %s", defaults)
	}
}

func TestZoneNodeAffinityIncludesZoneLabels(t *testing.T) {
	affinity := zoneNodeAffinity("us-west-2a")
	if affinity.NodeAffinity == nil || affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		t.Fatal("node affinity is not configured")
	}

	terms := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	if len(terms) != 2 {
		t.Fatalf("unexpected number of node selector terms: got=%d want=2", len(terms))
	}

	keys := map[string]struct{}{}
	for _, term := range terms {
		for _, expr := range term.MatchExpressions {
			keys[expr.Key] = struct{}{}
		}
	}

	if _, ok := keys[topologyZoneLabel]; !ok {
		t.Fatalf("missing required topology zone key: %s", topologyZoneLabel)
	}
	if _, ok := keys[legacyTopologyZoneLabel]; !ok {
		t.Fatalf("missing required legacy topology zone key: %s", legacyTopologyZoneLabel)
	}
}

func TestGetMultisiteSiteZoneAssignments(t *testing.T) {
	d := newTestDeploymentWithNodes(
		t,
		newReadyNode("node-1", "us-west-2d"),
		newReadyNode("node-2", "us-west-2a"),
		newReadyNode("node-3", "us-west-2c"),
	)

	assignments, err := d.getMultisiteSiteZoneAssignments(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"site1": "us-west-2a",
		"site2": "us-west-2c",
		"site3": "us-west-2d",
	}
	for siteName, zone := range expected {
		if assignments[siteName] != zone {
			t.Fatalf("unexpected zone assignment for %s: got=%s want=%s", siteName, assignments[siteName], zone)
		}
	}
}

func TestGetMultisiteSiteZoneAssignmentsFailsWhenInsufficientZones(t *testing.T) {
	d := newTestDeploymentWithNodes(
		t,
		newReadyNode("node-1", "us-west-2a"),
		newReadyNode("node-2", "us-west-2a"),
	)

	_, err := d.getMultisiteSiteZoneAssignments(context.Background(), 2)
	if err == nil {
		t.Fatal("expected error for insufficient zones")
	}
}
