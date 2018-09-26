package gosmtp

import (
	"github.com/stretchr/testify/assert"
	"log"
	"net"
	"os"
	"testing"
)

func init() {
	srv, err := NewServer(":4343", log.New(os.Stdout, "", log.LstdFlags))
	if err != nil {
		panic(err)
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()
}

func TestServer_Serve(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:4343")
	assert.NoError(t, err, "it should be possible to connect to the server")
	if err == nil {
		conn.Close()
	}
}
