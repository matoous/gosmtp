package main

import (
	"fmt"
	"github.com/matoous/gosmtp"
	"log"
	"net/mail"
	"os"
)

var configFileName string

func alwaysOkRcpt(adr *mail.Address) error {
	return nil
}

func main() {
	// set SMTP server and run
	server, err := gosmtp.NewServer(":3333", log.New(os.Stdout, "", log.LstdFlags))
	if err != nil {
		panic(err)
	}

	server.Hostname = "example.com"
	// authenticate all users
	server.Authenticator = func(peer *gosmtp.Peer, pw []byte) (bool, error) {
		fmt.Printf("User %s logged in\n", peer.Username)
		return true, nil
	}
	server.Handler = func(peer *gosmtp.Peer, envelope *gosmtp.Envelope) (string, error) {
		fmt.Printf("Mail peer:\n%#v\ndata:\n%#v\n", peer, envelope)
		return "1", nil
	}

	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
