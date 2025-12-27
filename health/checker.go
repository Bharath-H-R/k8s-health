package health

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DeploymentInfo struct {
	Name         string
	Namespace    string
	OwnerEmail   string
	OwnerDlEmail string
	Annotations  map[string]string
}

type FailedService struct {
	Deployment    DeploymentInfo
	FailureReason string
	PodLogs       string
	CheckTime     time.Time
}

type Checker struct {
	logTailLines int
}

func NewChecker() *Checker {
	return &Checker{
		logTailLines: 50,
	}
}

func (c *Checker) CheckDeploymentHealth(ctx context.Context, client *kubernetes.Clientset,
	dep DeploymentInfo) (bool, string, string, error) {

	// Get deployment pods
	pods, err := client.CoreV1().Pods(dep.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", dep.Name),
	})
	if err != nil {
		return false, "Failed to list pods", "", err
	}

	if len(pods.Items) == 0 {
		return false, "No pods found for deployment", "", nil
	}

	// Check each pod
	for _, pod := range pods.Items {
		// Check pod status
		if pod.Status.Phase != corev1.PodRunning {
			return false,
				fmt.Sprintf("Pod %s is not running (status: %s)", pod.Name, pod.Status.Phase),
				c.getPodLogs(ctx, client, pod),
				nil
		}

		// Check container statuses
		for _, container := range pod.Status.ContainerStatuses {
			if container.State.Waiting != nil {
				return false,
					fmt.Sprintf("Container %s is waiting: %s",
						container.Name, container.State.Waiting.Reason),
					c.getPodLogs(ctx, client, pod),
					nil
			}

			if container.State.Terminated != nil {
				return false,
					fmt.Sprintf("Container %s terminated: %s (exit code: %d)",
						container.Name, container.State.Terminated.Reason,
						container.State.Terminated.ExitCode),
					c.getPodLogs(ctx, client, pod),
					nil
			}

			if !container.Ready {
				// Check if there's a readiness probe failure
				if container.LastTerminationState.Terminated != nil {
					return false,
						fmt.Sprintf("Container %s not ready (last termination: %s)",
							container.Name, container.LastTerminationState.Terminated.Reason),
						c.getPodLogs(ctx, client, pod),
						nil
				}
				return false,
					fmt.Sprintf("Container %s not ready", container.Name),
					c.getPodLogs(ctx, client, pod),
					nil
			}
		}

		// Check for recent restarts
		for _, container := range pod.Status.ContainerStatuses {
			if container.RestartCount > 3 {
				return false,
					fmt.Sprintf("Container %s restarted %d times (possible crash loop)",
						container.Name, container.RestartCount),
					c.getPodLogs(ctx, client, pod),
					nil
			}
		}
	}

	return true, "", "", nil
}

func (c *Checker) getPodLogs(ctx context.Context, client *kubernetes.Clientset,
	pod corev1.Pod) string {

	if len(pod.Spec.Containers) == 0 {
		return "No containers in pod"
	}

	containerName := pod.Spec.Containers[0].Name
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: func(i int) *int64 { v := int64(i); return &v }(c.logTailLines),
	}

	logs, err := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions).Do(ctx).Raw()
	if err != nil {
		return fmt.Sprintf("Failed to get logs: %v", err)
	}

	return string(logs)
}
