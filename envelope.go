package gosmtp

import (
	"bufio"
	"bytes"
	"errors"
	"net/mail"
)

// Envelope represents a message envelope
type Envelope struct {
	MailFrom *mail.Address       // Envelope sender
	MailTo   []*mail.Address     // Envelope recipients
	Mail *mail.Message // Final message

	data     *bytes.Buffer // data stores the header and message body
	headers  map[string]string        // New headers added by server
}

func NewEnvelope() *Envelope {
	return &Envelope{
		MailTo: []*mail.Address{},
		data: bytes.NewBufferString(""),
		headers: make(map[string]string),
	}
}

// Close the envelope before handing it futher
func (e *Envelope) Close() (err error) {
	e.Mail, err = mail.ReadMessage(bufio.NewReader(e.data))
	if err != nil {
		return
	}

	for headerKey, headerValue := range e.headers {
		if e.Mail.Header.Get(headerKey) != "" {
			e.Mail.Header[headerKey] = append(e.Mail.Header[headerKey], headerValue)
		} else {
			e.Mail.Header[headerKey] = []string{headerValue}
		}
	}
	return
}

// IsSet returns if the envelope is set
func (e *Envelope) IsSet() bool {
	return e.MailFrom != nil
}

// Reader returns reader for envelope data
func (e *Envelope) Reader() *bytes.Reader {
	return bytes.NewReader(e.data.Bytes())
}

func (e *Envelope) Bytes() []byte {
	return e.data.Bytes()
}

// Reset resets envelope to initial state
func (e *Envelope) Reset() error {
	e.MailTo = []*mail.Address{}
	e.MailFrom = nil
	if e.data != nil {
		e.data.Reset()
	}
	for key, _ := range e.headers {
		delete(e.headers, key)
	}
	return nil
}

// AddRecipient adds recipient to envelope recipients
// returns error if maximum number of recipients is reached
func (e *Envelope) AddRecipient(rcpt *mail.Address) error {
	e.MailTo = append(e.MailTo, rcpt)
	return nil
}

func (e *Envelope) BeginData() error {
	if len(e.MailTo) == 0 {
		return errors.New("554 5.5.1 Error: no valid recipients")
	}
	e.data = bytes.NewBuffer([]byte{})
	return nil
}

// Write writes bytes to envelope buffer
func (e *Envelope) Write(line []byte) (int, error) {
	return e.data.Write(line)
}

// Write writes bytes to envelope buffer
func (e *Envelope) WriteString(line string) (int, error) {
	return e.data.WriteString(line)
}

// WriteLine writes data to envelope followed by new line
func (e *Envelope) WriteLine(line []byte) (int, error) {
	return e.data.Write(append(line, []byte("\r\n")...))
}