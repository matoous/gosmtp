package mta

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"gosmtp/config"
	"gosmtp/mail"
	"net"
	"sync"
	"time"

	"github.com/matoous/go-nanoid"
	"go.uber.org/zap"
)

/*
MailHandler is object on which func Handle(envelope, user) is called after the whole mail is received
MailHandler can be for example object which passes the email to MDA (mail delivery agent)
for remote delivery or to Dovercot to save the mail to the users inbox
*/
type MailHandler interface {
	Handle(envelope *mail.Envelope, user string) (id string, err error)
}

// ErrorRecipientNotFound is returned when the email is inbound but the user is not found
var ErrorRecipientNotFound = errors.New("Couldn't find recipient with given email address")

// ErrorRecipientsMailboxFull is returned when the user's mailbox is full
var ErrorRecipientsMailboxFull = errors.New("Recipients mailbox is full")

/*
ValidRcptFunc takes mail address as argument and returns nil if address is valid local recipient
and error otherwise
*/
type ValidRcptFunc func(mail.Address) error

/*
AuthFunc chacks if given email - password combination is valid
*/
type AuthFunc func(string, []byte) (bool, error)

/*
Server - full feature, RFC compliant, SMTP server implementation
*/
type Server struct {
	sync.Mutex
	Addr           string               // TCP address to listen on, ":25" if empty
	Hostname       string               // hostname, e.g. the domain which the server runs on
	tlcConfig      *tls.Config          // TLS configuration
	config         *config.ServerConfig // server configuration
	mailHandler    MailHandler          // object on which the Handle method is called when the whole mail is received
	limits         Limits               // server limits
	log            *zap.Logger          // servers logger
	authMechanisms []string             // announced authentication mechanisms
	authFunc       AuthFunc             // authentication function
	rcptValidator  ValidRcptFunc        // valid recipient checking function
	shuttingDown   bool                 // is the server shutting down?
}

// Auth sets the authentication function and authentication mechanisms which will be used
func (srv *Server) Auth(f AuthFunc, mechanisms ...string) error {
	if len(mechanisms) != 0 {
		// Check that all authenticatedUser configured authentication mechanisms are support
		for _, mech := range mechanisms {
			if !stringInSlice(mech, SupportedAuthMechanisms) {
				return fmt.Errorf("%v authentication mechanism is not supported", mech)
			}
		}
	} else {
		mechanisms = SupportedAuthMechanisms
	}
	srv.authMechanisms = mechanisms
	srv.authFunc = f
	return nil
}

/*
NewServer creates new server
*/
func NewServer(cfg *config.ServerConfig, handler MailHandler, rcptValidator ValidRcptFunc, limits ...Limits) (*Server, error) {
	// setup logger for server
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	s := &Server{
		config:        cfg,
		Addr:          cfg.HostPort,
		mailHandler:   handler,
		log:           logger,
		rcptValidator: rcptValidator,
		shuttingDown:  false,
	}
	// limits are optional, if no limits were provided, use the default ones
	if len(limits) == 1 {
		s.limits = limits[0]
	} else {
		s.limits = DefaultLimits
	}
	return s, nil
}

// ListenAndServe listens on the TCP network address and then
// calls Serve to handle requests on incoming connections.
// Connections are handled securely if it is available
func (srv *Server) ListenAndServe() error {
	// Load TCP listener
	if srv.config.TLSEnabled {
		var tlsConfig *tls.Config
		srv.config.Certificates = make([]tls.Certificate, 1)
		var cert tls.Certificate
		if srv.config.TLSCertFile != "" && srv.config.TLSKeyFile != "" {
			var err error
			cert, err = tls.LoadX509KeyPair(srv.config.TLSCertFile, srv.config.TLSKeyFile)
			if err != nil {
				srv.log.Error("no TLS: ", zap.Error(err))
				return err
			}
			tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			}
		} else {
			srv.log.Error("TLS enabled but no key and cert provided")
			return errors.New("not TLS key and cert")
		}
		l, err := tls.Listen("tcp", srv.config.HostPort, tlsConfig)
		if err != nil {
			return err
		}
		srv.log.Info("listening securely")
		return srv.Serve(l)
	}
	l, err := net.Listen("tcp", srv.config.HostPort)
	if err != nil {
		return err
	}
	return srv.Serve(l)
}

// Generate new context upon connection
func (srv *Server) newSession(conn net.Conn) *session {
	id, err := gonanoid.Nanoid()
	if err != nil {
		// generating nanoid shouldn't really fail, and if, panicing is OK
		panic(err)
	}
	s := &session{
		conn:     conn,
		id:       id,
		bufin:    bufio.NewScanner(conn),
		bufout:   bufio.NewWriter(conn),
		srv:      srv,
		envelope: &mail.Envelope{},
		start:    time.Now(),
		log:      srv.log,
	}
	// set split function so it reads to new line or 1024 bytes max
	s.bufin.Split(limitedLineSplitter)
	return s
}

// Serve incoming connections
// Creates new session for each connection and starts go routine to handle it
func (srv *Server) Serve(ln net.Listener) error {
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if netError, ok := err.(net.Error); ok && netError.Temporary() {
				srv.log.Error("temporary accept error", zap.Error(err))
				continue
			}
			return err
		}
		s := srv.newSession(conn)
		go s.Serve()
	}
}
