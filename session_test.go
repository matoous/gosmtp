package gosmtp

import (
	"fmt"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"strings"
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
	srv.Hostname = "test.com"

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()

	//cert, err := tls.LoadX509KeyPair("./cert.pem", "./key.pem")
	//config := &tls.Config{Certificates: []tls.Certificate{cert}}
	//srv2, err := NewServer(":4345", log.New(os.Stdout, "", log.LstdFlags))
	//if err != nil {
	//	panic(err)
	//}
	//
	//srv2.Handler = dummyHandle
	//srv2.RecipientChecker = dummyChecker
	//srv2.Hostname = "securetest.com"
	//srv2.TLSConfig = config
	//srv2.TLSOnly = true
	//
	//go func() {
	//	err := srv2.ListenAndServe()
	//	if err != nil {
	//		panic(err)
	//	}
	//}()
}

func TestSession_ExtensionBDAT(t *testing.T) {
	conn, err := smtp.Dial("localhost:4344")
	if err != nil {
		panic(err)
	}

	err = conn.Hello("it's me")
	assert.NoError(t, err)

	err = conn.Mail("pele@example.com")
	assert.NoError(t, err)

	err = conn.Rcpt("admin@admin.ws")
	assert.NoError(t, err)

	ok, _ := conn.Extension("CHUNKING")
	assert.True(t, ok, "CHUNKING extension is not supported but should be")

	writer := conn.Text.W
	data := []byte("From: pele@example.com\n" +
		"To: admin@admin.ws\n" +
		"Subject: Hello there\n\n" +
		"Hello!!!")
	data2 := []byte("it's me\n")
	writer.Write([]byte(fmt.Sprintf("BDAT %d\r\n", len(data))))
	writer.Write(data)
	writer.Flush()
	resp, err := conn.Text.ReadLine()
	assert.NoError(t, err, "didn't receive response to BDAT command")
	assert.True(t, strings.Contains(resp, "250"), "server sent other code than 250 as response to valid BDAT command")

	writer.Write([]byte(fmt.Sprintf("BDAT %d LAST\r\n", len(data2))))
	writer.Write(data2)
	writer.Flush()
	resp, err = conn.Text.ReadLine()
	assert.NoError(t, err, "didn't receive response to BDAT command")
	assert.True(t, strings.Contains(resp, "250"), "server sent other code than 250 as response to valid BDAT command")
}

func TestSession_ExtensionSTARTTLS(t *testing.T) {
	_, err := smtp.Dial("localhost:4344")
	if err != nil {
		panic(err)
	}
}

func TestSession_ExtensionHELP(t *testing.T) {
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

func TestSession_ExtensionPIPELINING(t *testing.T) {
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

func TestSession_ExtensionSMTPUTF8(t *testing.T) {
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
	}

	err = conn.Quit()
	assert.NoError(t, err, "error when sending RCPT")
}

func TestSession_handleEhlo(t *testing.T) {
	conn, err := smtp.Dial("localhost:4344")
	if err != nil {
		panic(err)
	}

	err = conn.Hello("it's me")
	assert.NoError(t, err, "error when sending hello")

	has, _ := conn.Extension("PIPELINING")
	assert.True(t, has, "gosmtp should support PIPELINING")

	has, _ = conn.Extension("8BITMIME")
	assert.True(t, has, "gosmtp should support 8BITMIME")

	has, _ = conn.Extension("CHUNKING")
	assert.True(t, has, "gosmtp should support CHUNKING")

	has, _ = conn.Extension("BINARYMIME")
	assert.True(t, has, "gosmtp should support BINARYMIME")

	has, _ = conn.Extension("SMTPUTF8")
	assert.True(t, has, "gosmtp should support SMTPUTF8")

	has, _ = conn.Extension("HELP")
	assert.True(t, has, "gosmtp should support HELP")

	has, _ = conn.Extension("SIZE")
	assert.True(t, has, "gosmtp should support SIZE")
}
