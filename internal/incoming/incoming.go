package incoming

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/go-pop3"
)

// Opt represents incoming mail processing options.
type Opt struct {
	Enabled      bool         `json:"enabled"`
	MailboxType  string       `json:"mailbox_type"`
	Mailboxes    []MailboxOpt `json:"mailboxes"`
	ScanInterval time.Duration
}

// MailboxOpt represents a POP3 mailbox configuration.
type MailboxOpt struct {
	UUID          string `json:"uuid"`
	Enabled       bool   `json:"enabled"`
	Type          string `json:"type"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	AuthProtocol  string `json:"auth_protocol"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	TLSEnabled    bool   `json:"tls_enabled"`
	TLSSkipVerify bool   `json:"tls_skip_verify"`
	ScanInterval  string `json:"scan_interval"`
}

// Manager handles incoming e-mail processing.
type Manager struct {
	opt       Opt
	db        *sqlx.DB
	log       *log.Logger
	queue     chan IncomingMail
	forwardCB func(IncomingMail) error
}

// IncomingMail represents a parsed incoming email.
type IncomingMail struct {
	ListID      int
	ListEmail   string
	From        string
	To          []string
	Subject     string
	Body        string
	HTMLBody    string
	ContentType string
	Headers     map[string][]string
	ReceivedAt  time.Time
}

// New returns a new instance of the incoming mail manager.
func New(opt Opt, db *sqlx.DB, forwardCB func(IncomingMail) error, lo *log.Logger) (*Manager, error) {
	m := &Manager{
		opt:       opt,
		db:        db,
		log:       lo,
		queue:     make(chan IncomingMail, 100),
		forwardCB: forwardCB,
	}

	return m, nil
}

// Run is a blocking function that processes incoming emails.
func (m *Manager) Run() {
	if !m.opt.Enabled {
		return
	}

	// Start the scanner for each enabled mailbox.
	for _, mb := range m.opt.Mailboxes {
		if !mb.Enabled {
			continue
		}

		// Parse scan interval.
		scanInterval, err := time.ParseDuration(mb.ScanInterval)
		if err != nil {
			m.log.Printf("invalid scan interval for mailbox %s: %v", mb.Host, err)
			continue
		}

		go m.runMailboxScanner(mb, scanInterval)
	}

	// Process emails from the queue.
	for mail := range m.queue {
		if err := m.forwardCB(mail); err != nil {
			m.log.Printf("error forwarding email: %v", err)
		}
	}
}

// runMailboxScanner runs a blocking loop that scans a mailbox at given intervals.
func (m *Manager) runMailboxScanner(mb MailboxOpt, scanInterval time.Duration) {
	for {
		m.log.Printf("scanning incoming mailbox %s for list emails", mb.Host)
		if err := m.scanMailbox(mb); err != nil {
			m.log.Printf("error scanning mailbox %s: %v", mb.Host, err)
		}

		time.Sleep(scanInterval)
	}
}

// scanMailbox scans a POP3 mailbox for incoming emails.
func (m *Manager) scanMailbox(mb MailboxOpt) error {
	client := pop3.New(pop3.Opt{
		Host:          mb.Host,
		Port:          mb.Port,
		TLSEnabled:    mb.TLSEnabled,
		TLSSkipVerify: mb.TLSSkipVerify,
	})

	c, err := client.NewConn()
	if err != nil {
		return fmt.Errorf("error connecting to mailbox: %w", err)
	}
	defer c.Quit()

	// Authenticate.
	if mb.AuthProtocol != "none" {
		if err := c.Auth(mb.Username, mb.Password); err != nil {
			return fmt.Errorf("error authenticating: %w", err)
		}
	}

	// Get the total number of messages on the server.
	count, _, err := c.Stat()
	if err != nil {
		return fmt.Errorf("error getting message count: %w", err)
	}

	// No messages.
	if count == 0 {
		return nil
	}

	// Get list email addresses from the database.
	listEmails, err := m.getListEmails()
	if err != nil {
		return fmt.Errorf("error getting list emails: %w", err)
	}

	if len(listEmails) == 0 {
		m.log.Printf("no lists with incoming email addresses configured")
		return nil
	}

	// Download and process messages.
	var processedIDs []int
	for id := 1; id <= count; id++ {
		// Retrieve the raw bytes of the message.
		b, err := c.RetrRaw(id)
		if err != nil {
			m.log.Printf("error retrieving message %d: %v", id, err)
			continue
		}

		// Parse the message.
		msg, err := m.parseMessage(b.Bytes(), listEmails)
		if err != nil {
			m.log.Printf("error parsing message %d: %v", id, err)
			continue
		}

		// If the message is not for any of our lists, skip it.
		if msg.ListID == 0 {
			continue
		}

		// Queue the message for forwarding.
		select {
		case m.queue <- *msg:
			processedIDs = append(processedIDs, id)
		default:
			m.log.Printf("queue full, skipping message %d", id)
		}
	}

	// Delete processed messages.
	for _, id := range processedIDs {
		if err := c.Dele(id); err != nil {
			m.log.Printf("error deleting message %d: %v", id, err)
		}
	}

	return nil
}

// getListEmails retrieves all list email addresses from the database.
func (m *Manager) getListEmails() (map[string]int, error) {
	rows, err := m.db.Query(`
		SELECT id, incoming_email FROM lists 
		WHERE incoming_email IS NOT NULL AND incoming_email != '' AND status = 'active'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listEmails := make(map[string]int)
	for rows.Next() {
		var (
			listID int
			email  string
		)
		if err := rows.Scan(&listID, &email); err != nil {
			return nil, err
		}
		listEmails[email] = listID
	}

	return listEmails, nil
}

// parseMessage parses an email message and extracts relevant information.
func (m *Manager) parseMessage(b []byte, listEmails map[string]int) (*IncomingMail, error) {
	// Create a mail reader directly from bytes.
	mr, err := mail.CreateReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("error creating mail reader: %w", err)
	}

	inMail := &IncomingMail{
		ReceivedAt: time.Now(),
		Headers:    make(map[string][]string),
	}

	// Extract headers.
	header := mr.Header
	if from, err := header.AddressList("From"); err == nil && len(from) > 0 {
		inMail.From = from[0].Address
	}

	if to, err := header.AddressList("To"); err == nil && len(to) > 0 {
		for _, addr := range to {
			inMail.To = append(inMail.To, addr.Address)
			// Check if this email is for one of our lists.
			if listID, ok := listEmails[addr.Address]; ok {
				inMail.ListID = listID
				inMail.ListEmail = addr.Address
			}
		}
	}

	// Also check Cc and Bcc headers for list addresses.
	if cc, err := header.AddressList("Cc"); err == nil && len(cc) > 0 {
		for _, addr := range cc {
			if listID, ok := listEmails[addr.Address]; ok && inMail.ListID == 0 {
				inMail.ListID = listID
				inMail.ListEmail = addr.Address
			}
		}
	}

	if subject, err := header.Subject(); err == nil {
		inMail.Subject = subject
	}

	// Copy all headers.
	for k, v := range header.Header.Map() {
		inMail.Headers[k] = v
	}

	// Extract body parts.
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("error reading message part: %w", err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			body, err := io.ReadAll(p.Body)
			if err != nil {
				return nil, fmt.Errorf("error reading body: %w", err)
			}

			if ct == "text/plain" {
				inMail.Body = string(body)
				inMail.ContentType = "plain"
			} else if ct == "text/html" {
				inMail.HTMLBody = string(body)
				if inMail.ContentType != "plain" {
					inMail.ContentType = "html"
				}
			}
		}
	}

	return inMail, nil
}

// GetStats returns statistics about the incoming mail processor.
func (m *Manager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":       m.opt.Enabled,
		"queue_size":    len(m.queue),
		"queue_cap":     cap(m.queue),
		"mailbox_count": len(m.opt.Mailboxes),
	}
}

// MarshalJSON implements json.Marshaler for logging.
func (mail IncomingMail) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"list_id":  mail.ListID,
		"from":     mail.From,
		"to":       mail.To,
		"subject":  mail.Subject,
		"received": mail.ReceivedAt,
	})
}
