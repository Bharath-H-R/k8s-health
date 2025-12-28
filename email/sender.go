package email

import (
    "bytes"
    "fmt"
    "html/template"
    "net/smtp"
    "os"
    "path/filepath"
    "time"
    
    "k8s-health-monitor/config"
    "k8s-health-monitor/health"
)

type Sender struct {
    config     config.SMTPConfig
    emailTemplate *template.Template
}

func NewSender(cfg config.SMTPConfig) (*Sender, error) {
    sender := &Sender{config: cfg}
    
    // Load email template
    err := sender.loadEmailTemplate()
    if err != nil {
        return nil, fmt.Errorf("failed to load email template: %w", err)
    }
    
    return sender, nil
}

func (s *Sender) loadEmailTemplate() error {
    // Try multiple locations for template file
    templatePaths := []string{
        "./email/template.html",
        "./template.html",
        "/app/email/template.html",
        "/app/template.html",
    }
    
    var templateContent string
    var found bool
    
    for _, path := range templatePaths {
        if content, err := os.ReadFile(path); err == nil {
            templateContent = string(content)
            found = true
            break
        }
    }
    
    if !found {
        // Fallback to embedded template
        return fmt.Errorf("email template not found in any location")
    }
    
    // Create template with custom functions
    tmpl, err := template.New("email").Funcs(template.FuncMap{
        "formatTime": func(t time.Time) string {
            return t.Format("Mon, 02 Jan 2006 15:04:05 MST")
        },
        "currentYear": func() int {
            return time.Now().Year()
        },
        "truncateLogs": func(logs string, maxLines int) string {
            lines := bytes.Split([]byte(logs), []byte("\n"))
            if len(lines) > maxLines {
                lines = lines[len(lines)-maxLines:]
            }
            return string(bytes.Join(lines, []byte("\n")))
        },
    }).Parse(templateContent)
    
    if err != nil {
        return fmt.Errorf("failed to parse email template: %w", err)
    }
    
    s.emailTemplate = tmpl
    return nil
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
    if s.emailTemplate == nil {
        return "", fmt.Errorf("email template not loaded")
    }
    
    // Create template data with additional fields
    templateData := struct {
        Deployment      health.DeploymentInfo
        FailureReason   string
        PodLogs         string
        CheckTime       time.Time
        LogTailLines    int
        ClusterName     string
        SupportEmail    string
        SlackChannel    string
    }{
        Deployment:    failedService.Deployment,
        FailureReason: failedService.FailureReason,
        PodLogs:       failedService.PodLogs,
        CheckTime:     failedService.CheckTime,
        LogTailLines:  50,
        ClusterName:   "EKS Production",
        SupportEmail:  "tech.infraengineers@godigit.com",
        SlackChannel:  "#tech-infra",
    }
    
    var buf bytes.Buffer
    if err := s.emailTemplate.Execute(&buf, templateData); err != nil {
        return "", fmt.Errorf("failed to execute email template: %w", err)
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
    headers["Importance"] = "high"
    
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
    result := ""
    for i, email := range emails {
        if i > 0 {
            result += ", "
        }
        result += email
    }
    return result
}
