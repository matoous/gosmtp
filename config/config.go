package config

import (
	"crypto/tls"
	"os"
	"sync"

	"github.com/jinzhu/configor"
	"go.uber.org/zap"
)

var C struct {
	sync.Mutex
	Me   string `required:"true"`
	Mode string `default:"debug"`

	DBDriver string `default:"sqlite3"`
	DBSource string `default:"sqlite.db"`

	StoreDriver string `default:"disk"`
	StoreSource string `default:"tmp/emails/"`

	Log          LoggerConfig
	ServerConfig ServerConfig

	LogConfig zap.Config
}

type LoggerConfig struct {
	LogFile string `default:"tmp/mlog.log"`
}

type ServerConfig struct {
	HostPort     string `default:":3333"` // TODO change to default smtp port
	TLSCertFile  string
	TLSKeyFile   string
	TLSEnabled   bool              `default:"false"`
	Certificates []tls.Certificate // the loaded certificates
	SayTime      bool              // report the time and date in the server banner
	LocalName    string            `default:"localhost"` // The local hostname to use in messages
	SftName      string            `default:"Mamail"`    // The software name to use in messages
	SftVersion   string            `default:"0.0.1"`
	Announce     string            `default:""` // extra stuff to announce in greeting banner
}

func Load(filename string) {
	C.Lock()
	defer C.Unlock()
	configor.Load(&C, filename)
	C.ServerConfig.LocalName = C.Me
}

func Default() error {
	C.Me = "me.me"
	C.Mode = "debug"

	if err := os.Mkdir("tmp/", os.ModePerm); err != nil {
		return err
	}

	C.DBDriver = "sqlite3"
	C.DBSource = "tmp/sqlite.db"

	C.StoreDriver = "disk"
	if err := os.Mkdir("tmp/emails", os.ModePerm); err != nil {
		return err
	}
	C.StoreSource = "tmp/emails"

	if err := os.Mkdir("tmp/log/", os.ModePerm); err != nil {
		return err
	}
	C.Log.LogFile = "tmp/log/mlog.log"

	sc := &C.ServerConfig
	sc.HostPort = ":3333"
	sc.TLSCertFile = ""
	sc.TLSKeyFile = ""
	sc.TLSEnabled = true
	sc.SayTime = true             // report the time and date in the server banner
	sc.LocalName = C.Me           // The local hostname to use in messages
	sc.SftName = "Mamail 0.0.1"   // The software name to use in messages
	sc.Announce = "welcome fella" // extra stuff to announce in greeting banner

	return nil
}

func GetDeliverdQueueBouncesLifetime() int {
	return 100
}

func GetDeliverdQueueLifetime() int {
	return 100
}
