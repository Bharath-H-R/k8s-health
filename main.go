package main

import (
	"context"
	"flag"
	"log"
	"time"

	"k8s-health-monitor/config"
	"k8s-health-monitor/email"
	"k8s-health-monitor/health"
	"k8s-health-monitor/kubernetes"
)

func main() {
	// Command line flags
	dryRun := flag.Bool("dry-run", false, "Dry run without sending emails")
	configPath := flag.String("config", "./config.yaml", "Path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize components
	ctx := context.Background()

	k8sClient, err := kubernetes.NewClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	scanner := kubernetes.NewScanner(k8sClient, cfg.ExcludedNamespaces)
	healthChecker := health.NewChecker()
	emailSender := email.NewSender(cfg.SMTPConfig)

	// Run health check
	log.Println("Starting Kubernetes service health check...")
	startTime := time.Now()

	deployments, err := scanner.ScanDeployments(ctx)
	if err != nil {
		log.Fatalf("Failed to scan deployments: %v", err)
	}

	// Check health for each deployment
	var failedServices []health.FailedService
	for _, dep := range deployments {
		if dep.OwnerEmail == "" || dep.OwnerDlEmail == "" {
			log.Printf("Warning: Deployment %s/%s missing owner annotations", dep.Namespace, dep.Name)
			continue
		}

		isHealthy, failureReason, podLogs, err := healthChecker.CheckDeploymentHealth(ctx, k8sClient, dep)
		if err != nil {
			log.Printf("Error checking health for %s/%s: %v", dep.Namespace, dep.Name, err)
			continue
		}

		if !isHealthy {
			failedServices = append(failedServices, health.FailedService{
				Deployment:    dep,
				FailureReason: failureReason,
				PodLogs:       podLogs,
				CheckTime:     time.Now(),
			})
		}
	}

	// Send notifications for failed services
	if len(failedServices) > 0 && !*dryRun {
		log.Printf("Found %d unhealthy services, sending notifications...", len(failedServices))

		for _, failedService := range failedServices {
			err := emailSender.SendHealthAlert(failedService)
			if err != nil {
				log.Printf("Failed to send email for %s/%s: %v",
					failedService.Deployment.Namespace,
					failedService.Deployment.Name,
					err)
			} else {
				log.Printf("Notification sent for %s/%s",
					failedService.Deployment.Namespace,
					failedService.Deployment.Name)
			}
			// Small delay to avoid overwhelming SMTP server
			time.Sleep(100 * time.Millisecond)
		}
	} else if *dryRun {
		log.Printf("Dry run: Found %d unhealthy services (no emails sent)", len(failedServices))
	} else {
		log.Println("All services are healthy!")
	}

	log.Printf("Health check completed in %v", time.Since(startTime))
}
