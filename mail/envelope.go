package mail

import (
	"bytes"
	"errors"
)

// Envelope represents a message envelope
type Envelope struct {
	// Envelope sender
	MailFrom Address
	// Envelope recipients
	MailTo []Address
	// Data stores the header and message body
	Data *bytes.Buffer
	// New headers added by server
	Headers []byte
}

// IsSet returns if the envelope is set
func (e Envelope) IsSet() bool {
	return e.MailFrom != ""
}

// Reader returns reader for envelope data
func (e Envelope) Reader() *bytes.Reader {
	return bytes.NewReader(e.Data.Bytes())
}

func (e Envelope) Bytes() []byte {
	return e.Data.Bytes()
}

// Reset resets envelope to initial state
func (e *Envelope) Reset() error {
	e.MailTo = nil
	e.MailFrom = ""
	if e.Data != nil {
		e.Data.Reset()
	}
	return nil
}

// AddRecipient adds recipient to envelope recipients
// returns error if maximum number of recipients is reached
func (e *Envelope) AddRecipient(rcpt Address) error {
	e.MailTo = append(e.MailTo, rcpt)
	return nil
}

func (e *Envelope) BeginData() error {
	if len(e.MailTo) == 0 {
		return errors.New("554 5.5.1 Error: no valid recipients")
	}
	e.Data = bytes.NewBuffer([]byte{})
	return nil
}

// Write writes bytes to envelope buffer
func (e *Envelope) Write(line []byte) (int, error) {
	return e.Data.Write(line)
}

// WriteLine writes data to envelope followed by new line
func (e *Envelope) WriteLine(line []byte) (int, error) {
	return e.Data.Write(append(line, []byte("\r\n")...))
}
