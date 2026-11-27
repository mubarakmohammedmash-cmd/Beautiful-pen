package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"
)

type ContactRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

const (
	TargetEmail = "muthman465@gmail.com"
	TargetPhone = "08133059491"
)

var (
	mu              sync.Mutex
	submissionMutex sync.Mutex
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/contact", handleContact)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/favicon.svg", serveStaticFile("favicon.svg", "image/svg+xml"))
	mux.HandleFunc("/sitemap.xml", serveStaticFile("sitemap.xml", "application/xml"))
	mux.HandleFunc("/robots.txt", serveStaticFile("robots.txt", "text/plain"))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      securityHeaders(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("==================================================")
	log.Printf("THE BEAUTIFUL PEN INTEGRATED SERVICES LIMITED - Server running on http://localhost:%s", port)
	log.Printf("Target Email: %s", TargetEmail)
	log.Printf("Target Phone: %s", TargetPhone)
	log.Printf("==================================================")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server launch failed: %v", err)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "index.html")
}

func handleContact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "Method not allowed"})
		return
	}

	body := http.MaxBytesReader(w, r.Body, 1024*1024)
	defer body.Close()

	var req ContactRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "Please provide valid enquiry details."})
		return
	}

	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "Invalid JSON request."})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Message = strings.TrimSpace(req.Message)

	if req.Name == "" || req.Phone == "" || req.Message == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "Name, phone number and message are required."})
		return
	}
	if len(req.Name) > 100 || len(req.Message) > 5000 || len(req.Subject) > 200 || len(req.Phone) > 30 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "One or more fields are too long."})
		return
	}
	if req.Email != "" && (len(req.Email) > 254 || !strings.Contains(req.Email, "@")) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "Please provide a valid email address."})
		return
	}
	if req.Subject == "" {
		req.Subject = "Website enquiry"
	}

	mu.Lock()
	defer mu.Unlock()

	if err := saveSubmission(req); err != nil {
		log.Printf("[SUBMISSION STORAGE ERROR] %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "The enquiry system is temporarily unavailable. Please call or WhatsApp us."})
		return
	}

	if err := dispatchEmail(req.Name, req.Email, req.Phone, req.Subject, req.Message); err != nil {
		log.Printf("[EMAIL ERROR] %v", err)
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "Your enquiry was received, but email delivery is temporarily unavailable."})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: "Thank you! Your enquiry has been sent successfully."})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: "Method not allowed"})
		return
	}
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: "Backend online"})
}

func dispatchEmail(senderName, senderEmail, senderPhone, subject, bodyContent string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	cleanLine := func(value string) string {
		value = strings.ReplaceAll(value, "\r", " ")
		return strings.ReplaceAll(value, "\n", " ")
	}
	senderName = cleanLine(strings.TrimSpace(senderName))
	senderEmail = cleanLine(strings.TrimSpace(senderEmail))
	senderPhone = cleanLine(strings.TrimSpace(senderPhone))
	subject = cleanLine(strings.TrimSpace(subject))
	if subject == "" {
		subject = "Website enquiry"
	}
	if smtpPort == "" {
		smtpPort = "587"
	}
	targetEmail := os.Getenv("TARGET_EMAIL")
	if targetEmail == "" {
		targetEmail = TargetEmail
	}
	if senderEmail == "" {
		senderEmail = "no-reply@thebeautifulpen.com"
	}

	if smtpHost == "" || smtpUser == "" || smtpPass == "" {
		log.Println("[DEV MODE] SMTP credentials missing; enquiry preview:")
		fmt.Printf("TO: %s\nFROM: %s <%s>\nPHONE: %s\nSUBJECT: %s\n%s\n",
			targetEmail, senderName, senderEmail, senderPhone, subject, bodyContent)
		return nil
	}

	var buffer bytes.Buffer
	mime := "MIME-version: 1.0;\nContent-Type: text/plain; charset=\"UTF-8\";\n\n"
	buffer.WriteString(fmt.Sprintf("From: %s <%s>\r\n", senderName, smtpUser))
	buffer.WriteString(fmt.Sprintf("To: %s\r\n", targetEmail))
	buffer.WriteString(fmt.Sprintf("Reply-To: %s\r\n", senderEmail))
	buffer.WriteString(fmt.Sprintf("Subject: [THE BEAUTIFUL PEN WEBSITE] %s\r\n", subject))
	buffer.WriteString(mime)
	buffer.WriteString(fmt.Sprintf("Sender Name: %s\n", senderName))
	buffer.WriteString(fmt.Sprintf("Sender Email: %s\n", senderEmail))
	buffer.WriteString(fmt.Sprintf("Sender Phone: %s\n", senderPhone))
	buffer.WriteString(fmt.Sprintf("Enquiry Subject: %s\n", subject))
	buffer.WriteString(fmt.Sprintf("Enquiry Message: %s\n\n", bodyContent))

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	address := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	return smtp.SendMail(address, auth, smtpUser, []string{targetEmail}, buffer.Bytes())
}

func saveSubmission(req ContactRequest) error {
	submissionMutex.Lock()
	defer submissionMutex.Unlock()

	file, err := os.OpenFile("submissions.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	record := struct {
		ReceivedAt time.Time `json:"received_at"`
		ContactRequest
	}{time.Now().UTC(), req}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	if err != nil {
		return err
	}
	return file.Sync()
}

func serveStaticFile(fileName, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, fileName)
	}
}
