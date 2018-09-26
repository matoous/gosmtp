package mta

import (
	"fmt"
	"gosmtp/config"
	"gosmtp/mail"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

type dummyMailHandler struct{}

// dummyMailHandler dumps the email to stdout upon receiving it
func (m *dummyMailHandler) Handle(envelope *mail.Envelope, user string) (id string, err error) {
	fmt.Printf("Dummy mail\nFrom: %#v\nTo: %#v\nNew headers: %s\nBody:\n%s\n", envelope.MailFrom, envelope.MailTo, envelope.Headers, envelope.Data)
	return "x", nil
}

var testHandler = &dummyMailHandler{}

// dummyValidator always returns nil as if the recipient was alright
func dummyValidator(rcpt mail.Address) error {
	return nil
}

var testConfig = &config.ServerConfig{
	HostPort:   ":7654",
	LocalName:  "me.me",
	SftVersion: "-0.0.1",
	SftName:    "Mamail test",
}

func TestNewServer(t *testing.T) {
	srv, err := NewServer(&config.ServerConfig{}, testHandler, dummyValidator)
	assert.Nil(t, err, "creating server shouldn't return error, at least for now")
	assert.Equal(t, DefaultLimits, srv.limits, "if limits are left empty, default limits should be used")
}

func TestServer_Serve(t *testing.T) {
	srv, err := NewServer(testConfig, testHandler, dummyValidator)
	if err != nil {
		panic(err)
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()

	conn, err := net.Dial("tcp", "localhost:7654")
	assert.NoError(t, err, "it should be possible to connect to the server")
	if err == nil {
		conn.Close()
	}
}
