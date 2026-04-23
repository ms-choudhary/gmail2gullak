package email

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/ms-choudhary/gmail2gullak/internal/gullak"
	"github.com/ms-choudhary/gmail2gullak/internal/models"
	"github.com/ms-choudhary/gmail2gullak/internal/parser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	tokenfile = ".token.json"
	statefile = ".last_read_state.json"
)

type Client interface {
	ProcessMessages(ctx context.Context, gc *gullak.Client) error
}

type GmailClient struct {
	service            *gmail.Service
	config             *oauth2.Config
	token              *oauth2.Token
	refreshTokenFailed bool
}

type Server struct {
	Config      *oauth2.Config
	EmailClient *GmailClient
}

func NewServer(credentialsFile string) (*Server, error) {
	credentials, err := ioutil.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("err: could not read credentials: %v", err)
	}

	config, err := google.ConfigFromJSON(credentials, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("err: could not parse credentials: %v", err)
	}
	return &Server{Config: config}, nil
}

func (s *Server) HandleLogin(w http.ResponseWriter, req *http.Request) {
	authURL := s.Config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	log.Print("starting login flow")

	http.Redirect(w, req, authURL, http.StatusMovedPermanently)
	return
}

func (s *Server) HandleStatus(w http.ResponseWriter, req *http.Request) {
	if s.EmailClient.refreshTokenFailed {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to refresh token")
		return
	}

	fmt.Fprintf(w, "ok")
}

func (s *Server) HandleOauthCallback(w http.ResponseWriter, req *http.Request) {
	code := req.URL.Query().Get("code")
	if code == "" {
		log.Print("oauthcallback: got empty authorization code")
		fmt.Fprintf(w, "got empty authorization code")
		return
	}

	token, err := s.Config.Exchange(context.TODO(), code)
	if err != nil {
		log.Printf("oauthcallback: failed to retrieve token: %v", err)
		fmt.Fprintf(w, "failed to retrieve token: %v", err)
		return
	}

	if err := saveToken(token); err != nil {
		log.Printf("oauthcallback: failed to save token: %v", err)
		fmt.Fprintf(w, "failed to save token: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><h1>Success!</h1></html>")
	log.Print("logged in successfully")
}

func writeJson(item any, file string) error {
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("cannot marshal item: %v", err)
	}

	return os.WriteFile(file, data, 0644)
}

func readToken() (*oauth2.Token, error) {
	var token oauth2.Token
	tokdata, err := ioutil.ReadFile(tokenfile)
	if err != nil {
		return nil, fmt.Errorf("could not read token: %v", err)
	}

	if err := json.Unmarshal(tokdata, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %v", err)
	}

	return &token, nil
}

func saveToken(token *oauth2.Token) error {
	if err := writeJson(*token, tokenfile); err != nil {
		return fmt.Errorf("failed to save token: %v", err)
	}
	return nil
}

func (s *Server) NewGmailClient(ctx context.Context) (Client, error) {
	token, err := readToken()
	if err != nil {
		return nil, err
	}

	client := s.Config.Client(ctx, token)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve gmail client: %v", err)
	}

	s.EmailClient = &GmailClient{
		service: srv,
		config:  s.Config,
		token:   token,
	}

	return s.EmailClient, nil
}

func (c *GmailClient) refreshToken(ctx context.Context) error {
	if c.token.Valid() {
		c.refreshTokenFailed = false
		return nil
	}

	tokenSource := c.config.TokenSource(ctx, c.token)
	newToken, err := tokenSource.Token()
	if err != nil {
		c.refreshTokenFailed = true
		return fmt.Errorf("failed to refresh token: %v", err)
	}

	if err := saveToken(newToken); err != nil {
		return err
	}

	c.token = newToken

	client := c.config.Client(ctx, newToken)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create new gmail service: %v", err)
	}
	c.service = srv

	c.refreshTokenFailed = false
	return nil
}

type ReadState struct {
	LastMessageID string `json:"last_message_id"`
}

func loadReadState() (*ReadState, error) {
	data, err := os.ReadFile(statefile)
	if err != nil {
		if os.IsNotExist(err) {
			return &ReadState{}, nil
		}
		return nil, fmt.Errorf("could not read state: %v", err)
	}

	var state ReadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("could not unmarshal state: %v", err)
	}

	return &state, nil
}

func saveReadState(state *ReadState) error {
	if err := writeJson(*state, statefile); err != nil {
		return fmt.Errorf("failed writing json: %v", err)
	}
	return nil
}

func (c *GmailClient) readMessagesFromLastReadState(state *ReadState) ([]*gmail.Message, error) {
	r, err := c.service.Users.Messages.List("me").MaxResults(100).Q("").Do()
	if err != nil {
		return []*gmail.Message{}, fmt.Errorf("could not retreive messages: %v", err)
	}

	if state.LastMessageID == "" {
		return r.Messages, nil
	}

	for i, m := range r.Messages {
		if m.Id == state.LastMessageID {
			return r.Messages[:i], nil
		}
	}

	return []*gmail.Message{}, fmt.Errorf("last message id not found: %v", state.LastMessageID)
}

func extractHeader(headers []*gmail.MessagePartHeader, name string) string {
	for _, header := range headers {
		if header.Name == name {
			return header.Value
		}
	}
	return name + " not found"
}

// decodeBase64URL decodes base64url encoded string
func decodeBase64URL(data string) ([]byte, error) {
	// Gmail API uses base64url encoding without padding
	// Go's base64 package expects standard base64, so we need to convert

	// Replace base64url characters with standard base64
	padding := (4 - len(data)%4) % 4
	data = data + strings.Repeat("=", padding) // Add padding
	data = strings.ReplaceAll(data, "-", "+")
	data = strings.ReplaceAll(data, "_", "/")

	return base64.StdEncoding.DecodeString(data)
}

func isPDFAttachment(part *gmail.MessagePart) bool {
	if part == nil {
		return false
	}

	isPDFMimeType := strings.EqualFold(part.MimeType, "application/pdf")
	hasPDFFilename := strings.HasSuffix(strings.ToLower(part.Filename), ".pdf")

	return (isPDFMimeType || hasPDFFilename) && part.Body != nil && part.Body.Data != ""
}

func decodePartBody(part *gmail.MessagePart) string {
	if part == nil || part.Body == nil || part.Body.Data == "" {
		return ""
	}

	data, err := decodeBase64URL(part.Body.Data)
	if err != nil {
		return ""
	}

	return string(data)
}

func collectPDFAttachmentParts(payload *gmail.MessagePart) []*gmail.MessagePart {
	if payload == nil {
		return nil
	}

	var parts []*gmail.MessagePart

	if isPDFAttachment(payload) {
		parts = append(parts, payload)
	}

	for _, part := range payload.Parts {
		parts = append(parts, collectPDFAttachmentParts(part)...)
	}

	return parts
}

// extractBody recursively extracts the plain-text body from the payload.
func extractBody(payload *gmail.MessagePart) string {
	if payload == nil {
		return ""
	}

	if payload.MimeType == "text/plain" {
		return decodePartBody(payload)
	}

	body := decodePartBody(payload)

	if payload.Parts != nil {
		for _, part := range payload.Parts {
			partBody := extractBody(part)

			if part.MimeType == "text/plain" && partBody != "" {
				return partBody
			}

			body += partBody
		}
	}

	return body
}

func (c *GmailClient) getPDFAttachmentData(messageID string, part *gmail.MessagePart) ([]byte, error) {
	if part == nil || part.Body == nil {
		return nil, fmt.Errorf("pdf attachment is missing body metadata")
	}

	if part.Body.Data != "" {
		data, err := decodeBase64URL(part.Body.Data)
		if err != nil {
			return nil, fmt.Errorf("could not decode inline attachment data: %v", err)
		}

		return data, nil
	}

	if part.Body.AttachmentId == "" {
		return nil, fmt.Errorf("pdf attachment is missing attachment id")
	}

	attachment, err := c.service.Users.Messages.Attachments.Get("me", messageID, part.Body.AttachmentId).Do()
	if err != nil {
		return nil, fmt.Errorf("could not fetch attachment data: %v", err)
	}

	data, err := decodeBase64URL(attachment.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode attachment data: %v", err)
	}

	return data, nil
}

func (c *GmailClient) extractPDFAttachments(messageID string, payload *gmail.MessagePart) []models.PDFAttachment {
	var attachments []models.PDFAttachment

	for _, part := range collectPDFAttachmentParts(payload) {
		fileData, err := c.getPDFAttachmentData(messageID, part)
		if err != nil {
			log.Printf("failed to load pdf attachment %q for message %s: %v", part.Filename, messageID, err)
			continue
		}

		attachments = append(attachments, models.PDFAttachment{
			Filename: part.Filename,
			MimeType: part.MimeType,
			FileData: fileData,
		})
	}

	return attachments
}

func (c *GmailClient) getMessageByID(id string) (models.Message, error) {
	msg, err := c.service.Users.Messages.Get("me", id).Do()
	if err != nil {
		return models.Message{}, fmt.Errorf("could not get message: %s: %v", id, err)
	}

	body := extractBody(msg.Payload)
	pdfAttachments := c.extractPDFAttachments(id, msg.Payload)

	return models.Message{
		ID:             id,
		Subject:        extractHeader(msg.Payload.Headers, "Subject"),
		From:           extractHeader(msg.Payload.Headers, "From"),
		Date:           extractHeader(msg.Payload.Headers, "Date"),
		Body:           body,
		PDFAttachments: pdfAttachments,
	}, nil
}

func (c *GmailClient) ProcessMessages(ctx context.Context, gullakClient *gullak.Client) error {
	if err := c.refreshToken(ctx); err != nil {
		return fmt.Errorf("failed to refresh token: %v", err)
	}

	state, err := loadReadState()
	if err != nil {
		log.Printf("could not read state: %v", err)
		state = &ReadState{}
	}

	defer saveReadState(state)

	messages, err := c.readMessagesFromLastReadState(state)
	if err != nil {
		return fmt.Errorf("could not read messages: %v", err)
	}

	if len(messages) == 0 {
		return nil
	}

	for i := len(messages) - 1; i >= 0; i-- {
		msg, err := c.getMessageByID(messages[i].Id)
		if err != nil {
			log.Print(err)
			continue
		}

		txn, err := parser.ParseTransaction(msg)
		if parser.IsNotTransaction(err) {
			// skip message if not transaction
			state.LastMessageID = messages[i].Id
			continue
		} else if err != nil {
			log.Printf("failed to parse transaction: %v", err)
			continue
		}

		if err := gullakClient.CreateTransaction(txn); err != nil {
			log.Printf("failed to create transaction: %v", err)
			continue
		}

		state.LastMessageID = messages[i].Id
	}

	return nil
}
