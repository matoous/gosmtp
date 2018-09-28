package gosmtp

import 	(
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type sessionState int

const (
	sessionStateInit sessionState = iota
	sessionStateGotMail
	sessionStateGotRcpt
	sessionStateReadyForData
	sessionStateGettingData
	sessionStateDataDone
	sessionStateAborted
	sessionStateWaitingForQuit
)

// Protocol represents the protocol used in the SMTP session
type Protocol string

const (
	SMTP Protocol = "SMTP" // SMTP - plain old SMTP
	ESMTP = "ESMTP" // ESMTP - Extended SMTP
)

// Peer represents the client connecting to the server
type Peer struct {
	HeloName        string
	HeloType        string
	Protocol        Protocol
	ServerName      string
	Username        string
	Authenticated   bool
	Addr            net.Addr
	TLS             *tls.ConnectionState
	AdditionalField map[string]interface{}
}

// session wraps underlying SMTP connection for easier handling
type session struct {
	conn   net.Conn       // connection
	bufio *bufio.ReadWriter // buffered input/output
	id     string         // email id

	envelope         *Envelope    // session envelope
	state            sessionState // session state
	badCommandsCount int          // amount of bad commands
	vrfyCount        int          // amount of vrfy commands received during current session
	start            time.Time    // start time of the session
	bodyType         string

	peer *Peer

	// tls info
	tls      bool // tls enabled
	tlsState tls.ConnectionState

	// hello info
	helloType int
	helloHost string
	helloSeen bool

	log *log.Logger // logger
	srv *Server     // serve handling this request
}

// Reset resets current session, happens upon MAIL, EHLO, HELO and RSET
func (s *session) Reset() {
	s.envelope.Reset()
	s.state = sessionStateInit
}

// DoneAndReset resets current after receiving DATA successfully
func (s *session) ReadLine() (string, error) {
	input, err := s.bufio.ReadString('\n')
	if err != nil {
		return "", err
	}
	// trim \r\n
	return input[:len(input)-2], nil
}

func (s *session) Out(msgs ...string) {
	// log
	s.log.Printf("INFO: returning msg: '%v'", msgs)

	s.conn.SetWriteDeadline(time.Now().Add(DefaultLimits.ReplyOut))
	for _, msg := range msgs {
		s.bufio.WriteString(msg)
		s.bufio.Write([]byte("\r\n"))
	}
	if err := s.bufio.Flush(); err != nil {
		s.log.Printf("ERROR: flush error: %s", err.Error())
		s.state = sessionStateAborted
	}
}

// Serve - serve given session
// scans command from input and handles given commands accordingly, any failure will result to immediate abort
// of connection
func (s *session) Serve() {
	defer s.conn.Close()

	// send welcome
	s.handleWelcome()

	// for each received command
	for {
		// TODO can we?
		if s.badCommandsCount >= s.srv.Limits.BadCmds {
			s.Out(Codes.FailMaxUnrecognizedCmd)
			s.state = sessionStateAborted
			break
		}
		line, err := s.ReadLine()
		if err != nil {
			s.log.Printf("ERROR: %s", err.Error())
			break
		}
		cmd, err := parseCommand(strings.TrimRightFunc(line, unicode.IsSpace))
		if err != nil {
			s.log.Printf("ERROR: unrecognized command: '%s'\n", strings.TrimRightFunc(line, unicode.IsSpace))
			s.Out(Codes.FailUnrecognizedCmd)
			s.badCommandsCount++
			continue
		}
		if s.state == sessionStateWaitingForQuit && cmd.commandCode != quitCmd {
			s.Out(Codes.FailBadSequence)
			s.badCommandsCount++
			continue
		}
		s.log.Printf("INFO: received command: '%s'", cmd.String())
		// select the right handler by commandCode
		handler := handlers[cmd.commandCode]
		handler(s, cmd)
		if s.state == sessionStateAborted {
			break
		}
		// TODO timeout might differ as per https://tools.ietf.org/html/rfc5321#section-4.5.3.2
		s.conn.SetReadDeadline(time.Now().Add(s.srv.Limits.CmdInput))
	}
}

// send Welcome upon new session creation
func (s *session) handleWelcome() {
	s.Out(fmt.Sprintf("220 %s ESMTP gomstp(0.0.1) I'm mr. Meeseeks, look at me!", s.peer.ServerName))
	/*
		The SMTP protocol allows a server to formally reject a mail session
		while still allowing the initial connection as follows: a 554
		response MAY be given in the initial connection opening message
		instead of the 220.  A server taking this approach MUST still wait
		for the client to send a QUIT (see Section 4.1.1.10) before closing
		the connection and SHOULD respond to any intervening commands with
		"503 bad sequence of commands".  Since an attempt to make an SMTP
		connection to such a system is probably in error, a server returning
		a 554 response on connection opening SHOULD provide enough
		information in the reply text to facilitate debugging of the sending
		system.
	*/
}

// handle Ehlo command
func handleEhlo(s *session, cmd *command) {
	s.Reset()

	s.helloSeen = true
	s.helloType = cmd.commandCode
	// TODO chec cmd args
	s.helloHost = cmd.arguments[0]
	// TODO check sending host (SPF)
	if s.srv.HeloChecker != nil {
		if err := s.srv.HeloChecker(s.peer, s.helloHost); err != nil {
			s.Out("550 " + err.Error())
		}
	}

	ehloResp := make([]string, 0, 10)

	ehloResp = append(ehloResp, fmt.Sprintf("250-%v hello %v", "mail.example", s.conn.RemoteAddr()))
	// https://tools.ietf.org/html/rfc6152
	ehloResp = append(ehloResp, "250-8BITMIME")
	// https://tools.ietf.org/html/rfc3030
	ehloResp = append(ehloResp, "250-CHUNKING")
	ehloResp = append(ehloResp, "250-BINARYMIME")
	// https://tools.ietf.org/html/rfc6531
	ehloResp = append(ehloResp, "250-SMTPUTF8")
	// https://tools.ietf.org/html/rfc2920
	ehloResp = append(ehloResp, "250-PIPELINING")
	// https://tools.ietf.org/html/rfc3207
	if s.srv.TLSConfig != nil { // do tls for this server
		if !s.tls { // already in tls stream
			ehloResp = append(ehloResp, "250-STARTTLS")
		}
	}
	// https://tools.ietf.org/html/rfc4954
	/*
		RFC4954 notes: A server implementation MUST
		implement a configuration in which it does NOT
		permit any plaintext password mechanisms, unless
		either the STARTTLS [SMTP-TLS] command has been negotiated...
	*/
	if len(s.srv.authMechanisms) != 0 && s.srv.TLSConfig != nil {
		ehloResp = append(ehloResp, "250-AUTH "+strings.Join(s.srv.authMechanisms, " "))
	}
	// https://tools.ietf.org/html/rfc821
	ehloResp = append(ehloResp, "250-HELP")
	// https://tools.ietf.org/html/rfc1870
	ehloResp = append(ehloResp, "250 SIZE 35882577") // from gmail TODO

	s.Out(ehloResp...)
}

// handle Helo command
func handleHelo(s *session, cmd *command) {
	s.Reset()
	s.helloSeen = true
	s.helloType = cmd.commandCode
	// TODO check cmd args
	s.helloHost = cmd.arguments[0]
	// TODO check sending host (SPF)
	if s.srv.HeloChecker != nil {
		if err := s.srv.HeloChecker(s.peer, s.helloHost); err != nil {
			s.Out("550 " + err.Error())
		}
	}

	s.Out(fmt.Sprintf("250 %v hello %v", "mail.example", s.conn.RemoteAddr()))
}

// start TLS
func handleStartTLS(s *session, cmd *command) {
	// already started TLS
	if s.tls {
		s.badCommandsCount++
		s.Out(Codes.FailBadSequence)
		return
	}

	if s.srv.TLSConfig == nil {
		s.Out(Codes.FailCmdNotSupported)
		return
	}

	s.Out(Codes.SuccessStartTLSCmd)

	// set timeout for TLS connection negotiation
	s.conn.SetDeadline(time.Now().Add(DefaultLimits.TLSSetup))
	secureConn := tls.Server(s.conn, s.srv.TLSConfig)

	// TLS handshake
	if err := secureConn.Handshake(); err != nil {
		s.log.Printf("ERROR: start tls: '%s'", err.Error())
		// TODO should we abort?
		s.Out(Codes.FailUndefinedSecurityStatus)
		return
	}

	// reset session
	s.Reset()
	s.conn = secureConn
	s.bufio = bufio.NewReadWriter(
		bufio.NewReader(s.conn),
		bufio.NewWriter(s.conn),
	)
	s.tls = true
	s.tlsState = secureConn.ConnectionState()
	s.peer.TLS = &s.tlsState
	s.state = sessionStateInit
}

func handleMail(s *session, cmd *command) {
	/*
		This command tells the SMTP-receiver that a new mail transaction is
		starting and to reset all its state tables and buffers, including any
		recipients or mail data.
	*/
	s.Reset()

	if !s.tls && s.srv.TLSOnly {
		s.Out(Codes.FailEncryptionNeeded)
		return
	}

	// require authentication if set in settings
	if len(s.srv.authMechanisms) != 0 && !s.peer.Authenticated {
		s.Out(Codes.FailAccessDenied)
		return
	}

	// nested mail command
	if s.envelope.IsSet() {
		s.Out(Codes.FailNestedMailCmd)
		return
	}

	args := cmd.arguments
	if len(args) == 0 {
		s.log.Print("DEBUG: Empty arguments for MAIL cmd")
		s.Out(Codes.FailInvalidAddress)
		return
	}

	// to lower and check if start with from: and is not empty
	from := strings.ToLower(strings.TrimSpace(args[0]))
	if from == "" || !strings.HasPrefix(from, "from:") {
		s.log.Print("DEBUG: Invalid address for MAIL cmd")
		s.Out(Codes.FailInvalidAddress)
		return
	}

	fromParts := strings.Split(from, ":")
	if len(fromParts) < 2 {
		s.Out(Codes.FailInvalidAddress)
		return
	}

	mailFrom, err := parseAddress(fromParts[1])
	if err != nil {
		s.Out(Codes.FailInvalidAddress)
		return
	}

	if s.srv.SenderChecker != nil {
		if err := s.srv.SenderChecker(s.peer, mailFrom); err != nil {
			s.Out(Codes.FailAccessDenied + " " + err.Error())
			return
		}
	}

	s.envelope.MailFrom = mailFrom
	args = args[1:]

	// extensions size
	if len(args) > 0 {
		for _, ext := range args {
			extValue := strings.Split(ext, "=")
			if len(extValue) != 2 {
				s.Out(Codes.FailInvalidAddress)
				return
			}
			switch strings.ToUpper(extValue[0]) {
			case "SIZE":
				size, err := strconv.ParseInt(extValue[1], 10, 64)
				if err != nil {
					s.Out(Codes.FailInvalidExtension)
					return
				}
				if int64(size) > DefaultLimits.MsgSize {
					s.Out(Codes.FailTooBig)
					return
				}
			case "BODY":
				// body-value ::= "7BIT" / "8BITMIME" / "BINARYMIME"
				s.bodyType = extValue[1]
			case "ALT-ADDRESS":
				/*
				   One optional parameter, ALT-ADDRESS, is added to the MAIL and
				   RCPT commands of SMTP.  ALT-ADDRESS specifies an all-ASCII
				   address which can be used as a substitute for the corresponding
				   primary (i18mail) address when downgrading.
				*/
			default:
				s.Out("555")
				s.envelope.MailFrom = nil
			}
		}
	}

	// validate FQN
	if err := IsFQN(s.envelope.MailFrom); err != "" {
		s.Out(err)
		return
	}

	switch s.state {
	case sessionStateGotRcpt:
		s.state = sessionStateReadyForData
	case sessionStateInit:
		s.state = sessionStateGotMail
	default:
		s.state = sessionStateAborted
		s.Out(Codes.FailBadSequence)
		return
	}
	s.Out(Codes.SuccessMailCmd)
}

func handleRcpt(s *session, cmd *command) {
	// if auth is required
	if len(s.srv.authMechanisms) != 0 && !s.peer.Authenticated {
		s.Out(Codes.FailAccessDenied)
		return
	}

	// HELO/EHLO needs to be first
	if !s.helloSeen {
		s.Out(Codes.FailBadSequence)
		return
	}

	// check recipients limit
	if len(s.envelope.MailTo) > DefaultLimits.MaxRcptCount {
		s.Out(Codes.ErrorTooManyRecipients)
		return
	}

	args := cmd.arguments
	if args == nil {
		s.Out(Codes.FailInvalidRecipient)
	}

	toParts := strings.Split(args[0], ":")
	if len(toParts) < 2 || strings.ToUpper(strings.TrimSpace(toParts[0])) != "TO" {
		s.Out(Codes.FailInvalidAddress)
		return
	}

	/*
		TODO
		Servers MUST be prepared to encounter a list of source
		routes in the forward-path, but they SHOULD ignore the routes or MAY
		decline to support the relaying they imply.
	*/
	// must be implemented - RFC5321
	if strings.ToLower(toParts[1]) == "<postmaster>" {
		toParts[1] = "<postmaster@" + s.peer.ServerName + ">"
	}

	rcpt, err := parseAddress(toParts[1])
	if err != nil {
		s.Out(Codes.FailInvalidAddress)
		return
	}

	// be relay for authenticated User
	if !s.peer.Authenticated {
		// check valid recipient if this email comes from outside
		err := s.srv.RecipientChecker(s.peer, rcpt)
		if err != nil {
			if err == ErrorRecipientNotFound {
				s.Out(Codes.FailMailboxDoesntExist)
				return
			}
			if err == ErrorRecipientsMailboxFull {
				s.Out(Codes.FailMailboxFull)
				return
			}
			s.Out(Codes.FailAccessDenied)
			return
		}
	}

	// Add to recipients
	err = s.envelope.AddRecipient(rcpt)
	if err != nil {
		s.Out(err.Error())
		return
	}

	// Change state
	switch s.state {
	case sessionStateGotMail:
		s.state = sessionStateReadyForData
	case sessionStateInit:
		s.state = sessionStateGotRcpt
	case sessionStateGotRcpt, sessionStateReadyForData:
	default:
		s.state = sessionStateAborted
		s.Out(Codes.FailBadSequence)
		return
	}
	s.Out(Codes.SuccessRcptCmd)
}

func handleVrfy(s *session, _ *command) {
	/*
		https://tools.ietf.org/html/rfc5336
		UTF8REPLY
	*/
	/*
		For the VRFY command, the string is a user name or a user name and
		domain (see below).  If a normal (i.e., 250) response is returned,
		the response MAY include the full name of the user and MUST include
		the mailbox of the user.  It MUST be in either of the following
		forms:

			User Name <local-part@domain>
			local-part@domain

	*/
	s.vrfyCount++

	// TODO implement vrfy
}

func handleData(s *session, cmd *command) {
	if s.bodyType == "BINARYMIME" {
		/*
			https://tools.ietf.org/html/rfc3030
			BINARYMIME cannot be used with the DATA command.  If a DATA command
			is issued after a MAIL command containing the body-value of
			"BINARYMIME", a 503 "Bad sequence of commands" response MUST be sent.
			The resulting state from this error condition is indeterminate and
			the transaction MUST be reset with the RSET command.
		*/
		s.Out(Codes.FailBadSequence)
		s.state = sessionStateAborted
		return
	}

	// envelope is ready for data
	if err := s.envelope.BeginData(); err != nil {
		s.Out(err.Error())
		return
	}

	// check if we are ready for data
	if s.state == sessionStateReadyForData {
		s.Out(Codes.SuccessDataCmd)
		s.state = sessionStateGettingData
	} else {
		s.Out(Codes.FailBadSequence)
		s.state = sessionStateAborted
		return
	}

	// set data input time limit
	s.conn.SetReadDeadline(time.Now().Add(DefaultLimits.MsgInput))

	// TODO https://tools.ietf.org/html/rfc5321#section-4.5.3.1.6
	// read data, stop on EOF or reaching maximum sizes
	var size int64
	for size < s.srv.Limits.MsgSize {
		line, err := s.bufio.ReadString('\n')
		if err != nil {
			s.Out(fmt.Sprintf(Codes.FailReadErrorDataCmd, err))
			s.state = sessionStateAborted
			return
		}
		line = strings.TrimSpace(line)
		if line == "." {
			break
		}
		size += int64(len(line))
		s.envelope.WriteString(line)
		s.envelope.Write([]byte("\r\n"))
	}

	// reading ended by reaching maximum size
	if size > s.srv.Limits.MsgSize {
		s.Out(Codes.FailTooBig)
		return
	}

	// add received header
	/*
		When forwarding a message into or out of the Internet environment, a
		gateway MUST prepend a Received: line, but it MUST NOT alter in any
		way a Received: line that is already in the header section.
	*/
	s.envelope.headers["Received"] = string(s.ReceivedHeader())

	// add Message-ID, is user is aut
	if s.peer.Authenticated {
		s.envelope.headers["Message-ID"] = fmt.Sprintf("Message-ID: <%d.%s@%s>\r\n", time.Now().Unix(), s.id, s.peer.ServerName)
	}

	// data done
	s.envelope.Close()
	s.state = sessionStateWaitingForQuit

	// add envelope to delivery system
	id, err := s.srv.Handler(s.peer, s.envelope)
	if err != nil {
		s.Out("451 temporary queue error")
	} else {
		s.Out(fmt.Sprintf("%v %s", Codes.SuccessMessageQueued, id))
	}

	// reset session
	s.Reset()
	return
}

// handleRset handle reset commands, reset currents session to beginning and empties the envelope
func handleRset(s *session, _ *command) {
	s.envelope.Reset()
	s.state = sessionStateInit
	s.Out(Codes.SuccessResetCmd)
}

func handleNoop(s *session, _ *command) {
	s.Out(Codes.SuccessNoopCmd)
}

func handleQuit(s *session, _ *command) {
	s.Out(Codes.SuccessQuitCmd)
	s.state = sessionStateAborted
	s.log.Printf("INFO: quit remote %s, server in %s", s.peer.Addr, time.Since(s.start))
}

func handleHelp(s *session, _ *command) {
	/*
		https://tools.ietf.org/html/rfc821
		This command causes the receiver to send helpful information
		to the sender of the HELP command.  The command may take an
		argument (e.g., any command name) and return more specific
		information as a response.
	*/
	s.Out(Codes.SuccessHelpCmd + " CaN yOu HelP Me PLeasE!")
}
func handleBdat(s *session, cmd *command) {
	args := cmd.arguments

	if s.state == sessionStateDataDone {
		/*
			Any BDAT command sent after the BDAT LAST is illegal and
			MUST be replied to with a 503 "Bad sequence of commands" reply code.
			The state resulting from this error is indeterminate.  A RSET command
			MUST be sent to clear the transaction before continuing.
		*/
		s.Out("503 Bad sequence of commands")
		return
	}

	last := false
	if len(args) == 0 {
		s.Out(Codes.FailUnrecognizedCmd) // TODO use the right code
		return
	}

	chunkSize64, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		s.Out(Codes.FailUnrecognizedCmd) // TODO use the right code
		s.badCommandsCount++
		return
	}

	if (len(args) > 1 && strings.ToUpper(args[1]) == "LAST") || chunkSize64 == 0 {
		last = true
	}

	s.log.Printf("INFO: received BDAT command, last: %t, data length: %d", last, chunkSize64)
	/*
		The message data is sent immediately after the trailing <CR>
		<LF> of the BDAT command line.  Once the receiver-SMTP receives the
		specified number of octets, it will return a 250 reply code.

		If a failure occurs after a BDAT command is
		received, the receiver-SMTP MUST accept and discard the associated
		message data before sending the appropriate 5XX or 4XX code.
	*/
	resp := make([]byte, chunkSize64)
	if n, err := s.bufio.Read(resp); err != nil {
		s.Out(fmt.Sprintf(Codes.FailReadErrorDataCmd, err))
		s.state = sessionStateAborted
		return
	} else if int64(n) != chunkSize64 {
		s.Out(fmt.Sprintf(Codes.FailReadErrorDataCmd, err))
		s.state = sessionStateAborted
		return
	}

	n, err := s.envelope.Write(resp)
	if int64(n) != chunkSize64 {
		s.Out(fmt.Sprintf(Codes.FailReadErrorDataCmd, err))
		s.state = sessionStateAborted
		return
	}

	if last {
		// data done
		s.Out(fmt.Sprintf("250 BDAT ok, BDAT finished, %d octets received", s.envelope.data.Len()))
		s.envelope.Close()
		s.state = sessionStateDataDone
	} else {
		/*
			A 250 response MUST be sent to each successful BDAT data block within
			a mail transaction.
		*/
		s.Out(fmt.Sprintf("250 BDAT ok, %d octets received", chunkSize64))
	}
}
func handleExpn(s *session, _ *command) {
	s.Out("252")
}

// Tempfail temporarily rejects the current SMTP command
func (s *session) Tempfail(cmd *command) {
	switch cmd.commandCode {
	case heloCmd, ehloCmd:
		s.Out("421 Not available now")
	case authCmd:
		s.Out("454 Temporary authentication failure")
	case mailCmd, rcptCmd, dataCmd:
		s.Out("450 Not available")
	}
}

// authMechanismValid checks if selected authentication mechanism is available
func (s *session) authMechanismValid(mech string) bool {
	mech = strings.ToUpper(mech)
	for _, m := range s.srv.authMechanisms {
		if mech == m {
			return true
		}
	}
	return false
}

func handleAuth(s *session, cmd *command) {
	if !s.tls {
		// Don't even allow unsecure authentication
		s.Out(Codes.FailEncryptionNeeded)
		return
	}

	// should not happen, some auth is always allowed
	if len(s.srv.authMechanisms) == 0 {
		s.Out(Codes.FailCmdNotSupported)
		// AUTH with no AUTH enabled counts as a
		// bad command. This deals with a few people
		// who spam AUTH requests at non-supporting
		// servers.
		s.badCommandsCount++
		return
	}

	// if authenticatedUser is already
	if s.peer.Authenticated {
		// RFC4954, section 4: After an AUTH
		// command has been successfully
		// completed, no more AUTH commands
		// may be issued in the same session.
		s.Out(Codes.FailBadSequence)
		return
	}

	args := cmd.arguments
	if len(args) == 0 {
		s.Out(Codes.FailMissingArgument)
		return
	}
	if !s.authMechanismValid(strings.ToUpper(args[0])) {
		s.Out(Codes.ErrorCmdParamNotImplemented)
		return
	}

	switch strings.ToUpper(args[0]) {
	case "PLAIN":
		s.handlePlainAuth(cmd)
	case "LOGIN":
		s.handleLoginAuth(cmd)
	default:
		s.Out(Codes.ErrorCmdParamNotImplemented)
		return
	}
}

func (s *session) ReceivedHeader() []byte {
	/*
		"Received:" header fields of messages originating from other
		environments may not conform exactly to this specification.  However,
		the most important use of Received: lines is for debugging mail
		faults, and this debugging can be severely hampered by well-meaning
		gateways that try to "fix" a Received: line.  As another consequence
		of trace header fields arising in non-SMTP environments, receiving
		systems MUST NOT reject mail based on the format of a trace header
		field and SHOULD be extremely robust in the light of unexpected
		information or formats in those header fields.

		The gateway SHOULD indicate the environment and protocol in the "via"
		clauses of Received header field(s) that it supplies.
	*/
	remoteIP := strings.Split(s.conn.RemoteAddr().String(), ":")[0]
	remotePort := strings.Split(s.conn.RemoteAddr().String(), ":")[1]
	remoteHost := "no reverse"
	if remoteHosts, err := net.LookupAddr(remoteIP); err == nil {
		remoteHost = remoteHosts[0]
	}
	localIP := strings.Split(s.conn.LocalAddr().String(), ":")[0]
	localHost := "no reverse"
	localHosts, err := net.LookupAddr(localIP)
	if err == nil {
		localHost = localHosts[0]
	}

	receivedHeader := bytes.NewBufferString("Received: from ")

	// host and IP
	receivedHeader.WriteString(remoteHost)
	receivedHeader.WriteString(" (")
	receivedHeader.WriteString(remoteHost)
	receivedHeader.WriteByte(':')
	receivedHeader.WriteString(remotePort)

	// authenticated
	if s.peer.Authenticated {
		receivedHeader.WriteString(" authenticated as ")
		receivedHeader.WriteString(s.peer.Username)
	}
	receivedHeader.WriteString(") ")

	// TLS
	if s.tls {
		receivedHeader.WriteString(tlsInfo(&s.tlsState))
		receivedHeader.WriteByte(' ')
	}

	// local
	receivedHeader.WriteString("by ")
	receivedHeader.WriteString(localIP)
	receivedHeader.WriteString(" (")
	receivedHeader.WriteString(localHost)
	receivedHeader.WriteByte(')')

	// proto
	if s.tls {
		receivedHeader.WriteString(" with ESMTPS; ")
	} else {
		receivedHeader.WriteString(" with SMTP; ")
	}

	// mamail version
	receivedHeader.WriteString(" gomstp(0.0.1)")

	receivedHeader.WriteString("; id ")
	receivedHeader.WriteString(s.id)

	// timestamp
	receivedHeader.WriteString("; ")
	receivedHeader.WriteString(time.Now().Format(time.RFC1123))
	receivedHeader.WriteString("\r\n")

	header := receivedHeader.Bytes()

	// fold header
	return wrap(header)
}

var handlers = []func(s *session, cmd *command){
	handleHelo,
	handleEhlo,
	handleQuit,
	handleRset,
	handleNoop,
	handleMail,
	handleRcpt,
	handleData,
	handleStartTLS,
	handleVrfy,
	handleExpn,
	handleHelp,
	handleAuth,
	handleBdat,
}

// http://www.rfc-base.org/txt/rfc-4408.txt
func checkHost(ip net.IPAddr, domain string, sender string) bool {

	return false
}
