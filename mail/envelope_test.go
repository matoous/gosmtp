package mail

import (
	"bytes"
	"github.com/go-errors/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEnvelope_AddRecipient(t *testing.T) {
	env := Envelope{}
	env.AddRecipient(Address("hello@example.com"))
	assert.Equal(t, len(env.MailTo), 1, "add recipient didn't work, recipient not added")
	assert.Equal(t, env.MailTo[0], Address("hello@example.com"), "add recipient didn't work, recipient added, but is wrong")
}

func TestEnvelope_IsSet(t *testing.T) {
	env := Envelope{}
	assert.Equal(t, env.IsSet(), false, "envelope is empty but acts as set")
	env.MailFrom = Address("hello@example.com")
	assert.Equal(t, env.IsSet(), true, "envelope is set but acts as empty")
}

func TestEnvelope_BeginData(t *testing.T) {
	env := Envelope{}
	assert.Equal(t, env.BeginData(), errors.New("554 5.5.1 Error: no valid recipients"), "envelope recipient list is empty but allows begin data")
	env.AddRecipient(Address("hello@example.com"))
	assert.Equal(t, env.BeginData(), nil, "envelope is ready to receive data but reports an error")
}

func TestEnvelope_Reset(t *testing.T) {
	env := Envelope{
		MailTo: []Address{
			"dzivjak@matous.me",
		},
		MailFrom: Address("dzivjak@matous.me"),
		Data:     bytes.NewBufferString("hello there"),
	}
	env.Reset()
	assert.Equal(t, env.Data.String(), "")
	assert.Equal(t, env.MailFrom, Address(""))
	assert.Equal(t, len(env.MailTo), 0)
	assert.Equal(t, env.IsSet(), false)
}
