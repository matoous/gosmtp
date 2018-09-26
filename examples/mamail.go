//package examples
//
//import (
//	"github.com/matoous/gosmtp"
//	"mamail/config"
//	"mamail/models"
//	"mamail/mta"
//	"net/mail"
//)
//
//var configFileName string
//
//func alwaysOkRcpt(adr *mail.Address) error {
//	return nil
//}
//
//func main() {
//	dummyMailStore := &gosmtp.DummyMailStore{}
//	// set SMTP server and run
//	server, err := mta.NewServer(&config.C.ServerConfig, dummyMailStore, alwaysOkRcpt)
//	if err != nil {
//		panic(err)
//	}
//	server.Auth(models.CheckUserLogin)
//	err = server.ListenAndServe()
//	panic(err)
//}
package examples