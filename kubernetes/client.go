// kubernetes/scanner.go
package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s-health-monitor/health"
)

type Scanner struct {
	client             *kubernetes.Clientset
	excludedNamespaces map[string]bool
}

func NewScanner(client *kubernetes.Clientset, excluded []string) *Scanner {
	excludedMap := make(map[string]bool)
	for _, ns := range excluded {
		excludedMap[ns] = true
	}

	return &Scanner{
		client:             client,
		excludedNamespaces: excludedMap,
	}
}

func (s *Scanner) ScanDeployments(ctx context.Context) ([]health.DeploymentInfo, error) {
	namespaces, err := s.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var deployments []health.DeploymentInfo

	for _, ns := range namespaces.Items {
		// Skip excluded namespaces
		if s.excludedNamespaces[ns.Name] {
			continue
		}

		// Get deployments in namespace
		deps, err := s.client.AppsV1().Deployments(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue // Log but continue with other namespaces
		}

		for _, dep := range deps.Items {
			// Extract owner annotations
			annotations := dep.GetAnnotations()
			ownerEmail := annotations["service_owner"]
			ownerDlEmail := annotations["owner_dl"]

			// Only include deployments with required annotations
			if ownerEmail != "" && ownerDlEmail != "" {
				deployments = append(deployments, health.DeploymentInfo{
					Name:         dep.Name,
					Namespace:    ns.Name,
					OwnerEmail:   ownerEmail,
					OwnerDlEmail: ownerDlEmail,
					Annotations:  annotations,
				})
			}
		}
	}

	return deployments, nil
}
