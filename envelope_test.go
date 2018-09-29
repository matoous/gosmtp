package gosmtp

import (
	"bytes"
	"net/mail"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvelope_AddRecipient(t *testing.T) {
	e1, _ := parseAddress("hello@example.com")
	env := Envelope{}
	env.AddRecipient(e1)
	assert.Equal(t, len(env.MailTo), 1, "add recipient should add recipient to envelope recipient list")
}

func TestEnvelope_IsSet(t *testing.T) {
	env := Envelope{}
	assert.Equal(t, env.IsSet(), false, "envelope is empty but acts as set")
	env.MailFrom, _ = parseAddress("hello@example.com")
	assert.Equal(t, env.IsSet(), true, "envelope is set but acts as empty")
}

func TestEnvelope_BeginData(t *testing.T) {
	env := NewEnvelope()
	assert.Error(t, env.BeginData(), "envelope recipient list is empty but allows begin data")
	e1, _ := parseAddress("hello@example.com")
	env.AddRecipient(e1)
	assert.NoError(t, env.BeginData(), "envelope is ready to receive data but reports an error")
}

func TestEnvelope_Reset(t *testing.T) {
	e1, _ := parseAddress("hello@example.com")
	e2, _ := parseAddress("helle@example.com")
	env := Envelope{
		MailTo:   []*mail.Address{e1},
		MailFrom: e2,
		data:     bytes.NewBufferString("hello there"),
	}
	env.Reset()
	assert.Equal(t, "", env.data.String(), "envelope data should be ampty after reset")
	assert.Nil(t, env.MailFrom, "mail from should be nil after reset")
	assert.Equal(t, 0, len(env.MailTo), "mail recipient should be empty after reset")
}
