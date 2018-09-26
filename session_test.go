package gosmtp

import (
	"fmt"
	"log"
	"net"
	"net/smtp"
	"os"
	"testing"
	"net/mail"

	"github.com/stretchr/testify/assert"
)

// dummyMailHandler dumps the email to stdout upon receiving it
func dummyHandle(peer *Peer, envelope *Envelope) (id string, err error) {
	fmt.Printf("Dummy mail\nFrom: %v\nTo: %v\nMail:\n%v\n", envelope.MailFrom, envelope.MailTo, envelope.Mail)
	return "x", nil
}

// dummyValidator always returns nil as if the recipient was alright
func dummyChecker(peer *Peer, mail *mail.Address) error {
	return nil
}

func init() {
	srv, err := NewServer(":4344", log.New(os.Stdout, "", log.LstdFlags))
	if err != nil {
		panic(err)
	}

	srv.Handler = dummyHandle
	srv.RecipientChecker = dummyChecker

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()
}

func TestSession_Extensions(t *testing.T) {
	SMTPUTF8(t)
	PIPELINING(t)
	HELP(t)
}

func HELP(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:4344")
	if err != nil {
		panic(err)
	}

	x := make([]byte, 1000)
	conn.Read(x)

	_, err = conn.Write([]byte("EHLO it's me\r\n"))
	assert.NoError(t, err, "error when sending HELLO")
	conn.Read(x)

	_, err = conn.Write([]byte("HELP\r\n"))
	assert.NoError(t, err, "error when sending PIPELINED commands")
	conn.Read(x)
	conn.Close()
}

func PIPELINING(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:4344")
	if err != nil {
		panic(err)
	}

	x := make([]byte, 1000)
	conn.Read(x)

	_, err = conn.Write([]byte("EHLO it's me\r\n"))
	assert.NoError(t, err, "error when sending HELLO")

	conn.Read(x)

	_, err = conn.Write([]byte("MAIL FROM:<test@test.te>\r\nRCPT TO:<test@test.te>\r\nRCPT TO:<test2@test2.te>\r\n"))
	assert.NoError(t, err, "error when sending PIPELINED commands")
	conn.Read(x)
	conn.Read(x)
	conn.Read(x)
	conn.Close()
}

func SMTPUTF8(t *testing.T) {
	conn, err := smtp.Dial("localhost:4344")
	if err != nil {
		panic(err)
	}

	err = conn.Hello("it's me")
	assert.NoError(t, err, "error when sending HELLO")

	err = conn.Mail("Pelé@example.com")
	assert.NoError(t, err, "error when sending UTF-8 MAIL")

	err = conn.Rcpt("admin@📙.ws")
	assert.NoError(t, err, "error when sending UTF-8 RCPT")

	err = conn.Rcpt("测试@测试.测试")
	assert.NoError(t, err, "error when sending UTF-8 RCPT")

	wr, err := conn.Data()
	assert.NoError(t, err, "error when sending DATA")

	written, err := wr.Write([]byte("Hello!"))
	assert.NoError(t, err, "error when sending message")
	assert.Equal(t, written, 6, "message sent only partialy")

	err = wr.Close()
	assert.NoError(t, err, "error when ending sending message")

	err = conn.Close()
	assert.NoError(t, err, "error when ending SMTP conversation")
}

func TestSession_Postmaster(t *testing.T) {
	conn, err := smtp.Dial("localhost:4344")
	if err != nil {
		panic(err)
	}

	err = conn.Hello("it's me")
	assert.NoError(t, err, "error when sending HELLO")

	err = conn.Mail("test@gmail.com")
	assert.NoError(t, err, "error when sending MAIL TO:postmaster")

	err = conn.Rcpt("postmaster")
	assert.NoError(t, err, "error when sending MAIL TO:postmaster")

	err = conn.Close()
	assert.NoError(t, err, "error when ending SMTP conversation")
}

func TestSession_Serve(t *testing.T) {
	conn, err := smtp.Dial("localhost:4344")
	if err != nil {
		panic(err)
	}

	err = conn.Hello("it's me")
	assert.NoError(t, err, "error when sending hello")

	err = conn.Quit()
	assert.NoError(t, err, "error when quiting the connection")

	conn, err = smtp.Dial("localhost:4344")
	if err != nil {
		panic(err)
	}

	err = conn.Hello("it's me second time")
	assert.NoError(t, err, "error when sending HELLO")

	err = conn.Mail("test@tsadasdasdasdsadsadest.te")
	assert.NoError(t, err, "error when sending MAIL")

	err = conn.Rcpt("test@other.te")
	assert.NoError(t, err, "error when sending RCPT")

	if err != nil {
		wr, err := conn.Data()
		assert.NoError(t, err, "error when sending DATA")

		written, err := wr.Write([]byte("Hello!"))
		assert.NoError(t, err, "error when sending message")
		assert.Equal(t, written, 6, "message sent only partialy")

		err = wr.Close()
		assert.NoError(t, err, "error when ending sending message")
	}
}
