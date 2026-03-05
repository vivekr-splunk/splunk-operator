package testenv

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestEffectiveCriticalReadinessTimeoutCapsLargeTimeout(t *testing.T) {
	t.Setenv("SPLUNK_OPERATOR_READY_CHECK_TIMEOUT_SECONDS", "")

	deployment := &Deployment{testTimeout: 6 * time.Hour}
	testEnv := &TestCaseEnv{Log: logf.Log.WithName("verificationutils-timeout-cap")}
	got := effectiveCriticalReadinessTimeout(deployment, testEnv)

	if got != defaultCriticalReadinessTimeout {
		t.Fatalf("expected timeout %s, got %s", defaultCriticalReadinessTimeout, got)
	}
}

func TestEffectiveCriticalReadinessTimeoutHonorsLowerEnvOverride(t *testing.T) {
	t.Setenv("SPLUNK_OPERATOR_READY_CHECK_TIMEOUT_SECONDS", "120")

	deployment := &Deployment{testTimeout: 2 * time.Hour}
	testEnv := &TestCaseEnv{Log: logf.Log.WithName("verificationutils-timeout-env")}
	got := effectiveCriticalReadinessTimeout(deployment, testEnv)

	if got != 120*time.Second {
		t.Fatalf("expected timeout %s, got %s", 120*time.Second, got)
	}
}

func TestEffectiveCriticalReadinessTimeoutIgnoresInvalidEnvOverride(t *testing.T) {
	t.Setenv("SPLUNK_OPERATOR_READY_CHECK_TIMEOUT_SECONDS", "invalid")

	deployment := &Deployment{testTimeout: 20 * time.Minute}
	testEnv := &TestCaseEnv{Log: logf.Log.WithName("verificationutils-timeout-invalid")}
	got := effectiveCriticalReadinessTimeout(deployment, testEnv)

	if got != 20*time.Minute {
		t.Fatalf("expected timeout %s, got %s", 20*time.Minute, got)
	}
}

func TestTerminalContainerStatusErrorDetectsTerminalWaitingReason(t *testing.T) {
	status := corev1.ContainerStatus{
		Name: "splunk",
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Reason:  "CrashLoopBackOff",
				Message: "back-off restarting failed container",
			},
		},
	}

	err := terminalContainerStatusError("splunk-smoke-cluster-manager-0", status, "container")
	if err == nil {
		t.Fatalf("expected terminal status error, got nil")
	}
	if !strings.Contains(err.Error(), "CrashLoopBackOff") {
		t.Fatalf("expected error to include waiting reason, got %q", err.Error())
	}
}

func TestFailFastOnTerminalPodStatesDetectsImagePullFailure(t *testing.T) {
	if err := corev1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatalf("failed to add corev1 scheme: %v", err)
	}

	namespace := "smoke-test-ns"
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "splunk-smoke-fail-cluster-manager-0",
				Namespace: namespace,
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "splunk",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "ErrImagePull",
								Message: "manifest unknown",
							},
						},
					},
				},
			},
		},
	).Build()

	deployment := &Deployment{
		name:        "smoke-fail",
		testTimeout: time.Hour,
	}
	testEnv := &TestCaseEnv{
		kubeClient: fakeClient,
		name:       namespace,
		Log:        logf.Log.WithName("verificationutils-failfast"),
	}

	err := failFastOnTerminalPodStates(context.Background(), deployment, testEnv)
	if err == nil {
		t.Fatalf("expected fail-fast error, got nil")
	}
	if !strings.Contains(err.Error(), "ErrImagePull") {
		t.Fatalf("expected fail-fast error to include ErrImagePull, got %q", err.Error())
	}
}
