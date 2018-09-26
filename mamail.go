package main

import (
	"gosmtp/config"
	"gosmtp/mail"
	"gosmtp/mta"
	"mamail/models"
)

var configFileName string

func alwaysOkRcpt(adr mail.Address) error {
	return nil
}

func main() {
	dummyMailStore := &mta.DummyMailStore{}
	// set SMTP server and run
	server, err := mta.NewServer(&config.C.ServerConfig, dummyMailStore, alwaysOkRcpt)
	if err != nil {
		panic(err)
	}
	server.Auth(models.CheckUserLogin)
	err = server.ListenAndServe()
	panic(err)
}
