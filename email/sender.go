package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

	"k8s-health-monitor/config"
	"k8s-health-monitor/health"
)

type Sender struct {
	config config.SMTPConfig
}

func NewSender(cfg config.SMTPConfig) *Sender {
	return &Sender{config: cfg}
}

func (s *Sender) SendHealthAlert(failedService health.FailedService) error {
	// Prepare email content
	subject := fmt.Sprintf("[URGENT] Service Health Alert: %s/%s is DOWN",
		failedService.Deployment.Namespace,
		failedService.Deployment.Name)

	// Generate HTML body
	htmlBody, err := s.generateHTMLBody(failedService)
	if err != nil {
		return fmt.Errorf("failed to generate email body: %w", err)
	}

	// Prepare recipients
	to := []string{failedService.Deployment.OwnerEmail}
	cc := []string{
		failedService.Deployment.OwnerDlEmail,
		"tech.infraengineers@godigit.com",
	}

	// Send email
	return s.sendEmail(to, cc, subject, htmlBody)
}

func (s *Sender) generateHTMLBody(failedService health.FailedService) (string, error) {
	const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: 'Segoe UI', Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { 
            background: linear-gradient(135deg, #0066cc 0%, #004d99 100%);
            color: white; 
            padding: 30px; 
            border-radius: 10px 10px 0 0;
            text-align: center;
        }
        .header h1 { margin: 0; font-size: 24px; }
        .header .subtitle { opacity: 0.9; margin-top: 5px; }
        .content { 
            background: #f8f9fa; 
            padding: 30px; 
            border-radius: 0 0 10px 10px;
            border: 1px solid #dee2e6;
            border-top: none;
        }
        .alert-box { 
            background: #fff3cd; 
            border: 1px solid #ffeaa7; 
            border-left: 5px solid #f39c12;
            padding: 15px; 
            margin: 20px 0; 
            border-radius: 5px;
        }
        .info-box { 
            background: #d1ecf1; 
            border: 1px solid #bee5eb; 
            padding: 15px; 
            margin: 20px 0; 
            border-radius: 5px;
        }
        .details-table { 
            width: 100%; 
            border-collapse: collapse; 
            margin: 20px 0;
            background: white;
        }
        .details-table th, .details-table td { 
            padding: 12px 15px; 
            text-align: left; 
            border-bottom: 1px solid #dee2e6;
        }
        .details-table th { 
            background: #f8f9fa; 
            font-weight: 600;
            color: #495057;
        }
        .status-badge { 
            display: inline-block; 
            padding: 4px 12px; 
            border-radius: 20px; 
            font-size: 12px; 
            font-weight: 600;
        }
        .status-critical { background: #dc3545; color: white; }
        .logs-box { 
            background: #1e1e1e; 
            color: #d4d4d4; 
            padding: 15px; 
            font-family: 'Consolas', monospace;
            font-size: 12px; 
            border-radius: 5px; 
            overflow-x: auto;
            margin: 20px 0;
            max-height: 300px;
            overflow-y: auto;
        }
        .footer { 
            text-align: center; 
            margin-top: 30px; 
            padding-top: 20px; 
            border-top: 1px solid #dee2e6;
            color: #6c757d; 
            font-size: 12px;
        }
        .digit-logo { 
            color: white; 
            font-weight: bold; 
            font-size: 20px;
            display: inline-block;
            margin-bottom: 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="digit-logo">DIGIT INSURANCE</div>
            <h1>🚨 Kubernetes Service Health Alert</h1>
            <div class="subtitle">Automated Monitoring System | Tech Infrastructure Team</div>
        </div>
        
        <div class="content">
            <div class="alert-box">
                <strong>CRITICAL:</strong> One of your services has failed health checks and requires immediate attention.
            </div>
            
            <h2>Service Details</h2>
            <table class="details-table">
                <tr>
                    <th width="30%">Service Name</th>
                    <td><strong>{{.Deployment.Name}}</strong></td>
                </tr>
                <tr>
                    <th>Namespace</th>
                    <td>{{.Deployment.Namespace}}</td>
                </tr>
                <tr>
                    <th>Owner</th>
                    <td>{{.Deployment.OwnerEmail}}</td>
                </tr>
                <tr>
                    <th>Team DL</th>
                    <td>{{.Deployment.OwnerDlEmail}}</td>
                </tr>
                <tr>
                    <th>Status</th>
                    <td><span class="status-badge status-critical">CRITICAL - SERVICE DOWN</span></td>
                </tr>
                <tr>
                    <th>Detection Time</th>
                    <td>{{.CheckTime.Format "Mon, 02 Jan 2006 15:04:05 MST"}}</td>
                </tr>
            </table>
            
            <h2>Failure Analysis</h2>
            <div class="info-box">
                <strong>Root Cause:</strong> {{.FailureReason}}
                <br><br>
                <strong>Impact:</strong> Service is unavailable or not responding to health checks.
                This may affect dependent services and customer experience.
            </div>
            
            {{if .PodLogs}}
            <h2>Recent Logs (Last 50 Lines)</h2>
            <div class="logs-box">
                <pre>{{.PodLogs}}</pre>
            </div>
            {{end}}
            
            <h2>Required Actions</h2>
            <ol>
                <li>Immediately investigate the service health in the EKS cluster</li>
                <li>Check pod status: <code>kubectl get pods -n {{.Deployment.Namespace}} -l app={{.Deployment.Name}}</code></li>
                <li>Review recent deployments or configuration changes</li>
                <li>Check resource utilization (CPU/Memory)</li>
                <li>Verify network connectivity and dependencies</li>
                <li>Update the Tech Infrastructure team once resolved</li>
            </ol>
            
            <div class="info-box">
                <strong>Support:</strong> For immediate assistance, contact the Tech Infrastructure team at 
                <a href="mailto:tech.infraengineers@godigit.com">tech.infraengineers@godigit.com</a> or 
                join the #tech-infra Slack channel.
            </div>
        </div>
        
        <div class="footer">
            <p>
                This is an automated alert from Digit Insurance Kubernetes Monitoring System.<br>
                Monitoring Interval: Daily at 8:00 AM (Monday-Friday)<br>
                Cluster: EKS Production | Environment: {{.Deployment.Namespace}}<br>
                © {{.CheckTime.Format "2006"}} Digit Insurance. All rights reserved.
            </p>
        </div>
    </div>
</body>
</html>`

	tmpl, err := template.New("email").Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, failedService); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *Sender) sendEmail(to, cc []string, subject, body string) error {
	// Prepare email headers
	headers := make(map[string]string)
	headers["From"] = s.config.From
	headers["To"] = to[0]
	headers["Cc"] = joinEmails(cc)
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"
	headers["X-Priority"] = "1" // High priority
	headers["X-MSMail-Priority"] = "High"

	// Build message
	var message bytes.Buffer
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(body)

	// Send email via SMTP
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	if s.config.NoAuth {
		// For whitelisted server without auth
		return smtp.SendMail(addr, nil, s.config.From, append(to, cc...), message.Bytes())
	} else {
		// For servers requiring auth (if needed in future)
		// auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		// return smtp.SendMail(addr, auth, s.config.From, append(to, cc...), message.Bytes())
		return smtp.SendMail(addr, nil, s.config.From, append(to, cc...), message.Bytes())
	}
}

func joinEmails(emails []string) string {
	return fmt.Sprintf("%s", emails)
}
