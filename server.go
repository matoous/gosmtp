package gosmtp

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/mail"
	"sync"
	"time"

	"github.com/matoous/go-nanoid"
)

/*
MailHandler is object on which func Handle(envelope, user) is called after the whole mail is received
MailHandler can be for example object which passes the email to MDA (mail delivery agent)
for remote delivery or to Dovercot to save the mail to the users inbox
*/
type MailHandler interface {
	Handle(envelope *Envelope, user string) (id string, err error)
}

// ErrorRecipientNotFound is returned when the email is inbound but the user is not found
var ErrorRecipientNotFound = errors.New("Couldn't find recipient with given email address")

// ErrorRecipientsMailboxFull is returned when the user's mailbox is full
var ErrorRecipientsMailboxFull = errors.New("Recipients mailbox is full")

/*
Server - full feature, RFC compliant, SMTP server implementation
*/
type Server struct {
	sync.Mutex
	Addr           string      // TCP address to listen on, ":25" if empty
	Hostname       string      // hostname, e.g. the domain which the server runs on
	TLSConfig      *tls.Config // TLS configuration
	TLSOnly        bool
	log            *log.Logger          // servers logger
	authMechanisms []string             // announced authentication mechanisms

	shuttingDown bool // is the server shutting down?

	// Limits
	Limits Limits

	// New e-mails are handed off to this function.
	// Can be left empty for a NOOP server.
	// Returned ID should be ID of the queued email if the email is put into outgoing queue
	// If an error is returned, it will be reported in the SMTP session.
	Handler func(peer *Peer, env *Envelope) (string, error)

	// Enable PLAIN/LOGIN authentication
	Authenticator func(peer *Peer, password []byte) (bool, error)

	// Enable various checks during the SMTP session.
	// Can be left empty for no restrictions.
	// If an error is returned, it will be reported in the SMTP session.
	// Use the Error struct for access to error codes.
	ConnectionChecker func(peer *Peer) error              // Called upon new connection.
	HeloChecker       func(peer *Peer, name string) error // Called after HELO/EHLO.
	SenderChecker     func(peer *Peer, addr *mail.Address) error // Called after MAIL FROM.
	RecipientChecker  func(peer *Peer, addr *mail.Address) error // Called after each RCPT TO.
}

// Auth sets the authentication function and authentication mechanisms which will be used
func (srv *Server) Auth(f func(*Peer, []byte) (bool, error), mechanisms ...string) error {
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
	srv.Authenticator = f
	return nil
}

/*
NewServer creates new server
*/
func NewServer(port string, logger *log.Logger, limits ...Limits) (*Server, error) {
	s := &Server{
		Addr:         port,
		log:          logger,
		shuttingDown: false,
	}
	// limits are optional, if no limits were provided, use the default ones
	if len(limits) == 1 {
		s.Limits = limits[0]
	} else {
		s.Limits = DefaultLimits
	}
	return s, nil
}

// ListenAndServe listens on the TCP network address and then
// calls Serve to handle requests on incoming connections.
// Connections are handled securely if it is available
func (srv *Server) ListenAndServe() error {
	if srv.TLSConfig != nil {
		l, err := tls.Listen("tcp", srv.Addr, srv.TLSConfig)
		if err != nil {
			return err
		}
		return srv.Serve(l)
	}
	l, err := net.Listen("tcp", srv.Addr)
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
		id:       id,
		conn:     conn,
		bufio:    bufio.NewReadWriter(
			bufio.NewReader(conn),
			bufio.NewWriter(conn),
		),
		srv:      srv,
		envelope: NewEnvelope(),
		start:    time.Now(),
		log:      srv.log,
		peer: &Peer{
			Addr:       conn.RemoteAddr(),
			ServerName: srv.Hostname,
		},
	}

	var tlsConn *tls.Conn
	tlsConn, s.tls = conn.(*tls.Conn)
	if s.tls {
		state := tlsConn.ConnectionState()
		s.peer.TLS = &state
	}

	// set split function so it reads to new line or 1024 bytes max
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
				srv.log.Printf("temporary accept error %s", err.Error())
				continue
			}
			return err
		}
		s := srv.newSession(conn)
		go s.Serve()
	}
}
