package mta

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"gosmtp/config"
	"gosmtp/mail"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
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

// session wraps underlying SMTP connection for easier handling
type session struct {
	conn             net.Conn       // connection
	bufin            *bufio.Scanner // scanner
	bufout           *bufio.Writer  // writer
	id               string         // email id
	envelope         *mail.Envelope // session envelope
	state            sessionState   // session state
	badCommandsCount int            // amount of bad commands
	vrfyCount        int            // amount of vrfy commands received during current session
	start            time.Time      // start time of the session
	bodyType         string

	// tls info
	tls      bool // tls enabled
	tlsState tls.ConnectionState

	// hello info
	helloType int
	helloHost string
	helloSeen bool

	// authentication info
	authenticated     bool   // true after successful auth dialog
	authenticatedUser string // User authenticated for current session

	log *zap.Logger // logger
	srv *Server     // serve handling this request
}

// Reset resets current session, happens upon MAIL, EHLO, HELO and RSET
func (s *session) Reset() {
	s.envelope.Reset()
	s.state = sessionStateInit
}

// DoneAndReset resets current after receiving DATA successfully
func (s *session) DoneAndReset() {
	s.envelope = &mail.Envelope{}
	s.state = sessionStateInit
}

func (s *session) Out(msgs ...string) {
	// log
	s.log.Debug("returning msg:", zap.Strings("msgs", msgs))

	s.conn.SetWriteDeadline(time.Now().Add(DefaultLimits.ReplyOut))
	for _, msg := range msgs {
		s.bufout.WriteString(msg)
		s.bufout.Write([]byte("\r\n"))
	}
	if err := s.bufout.Flush(); err != nil {
		s.log.Error("flush", zap.Error(err))
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
	for s.bufin.Scan() {
		cmd, err := parseCommand(s.bufin.Text())
		if err != nil {
			s.Out(mail.Codes.FailUnrecognizedCmd)
			s.badCommandsCount++
			continue
		}
		if s.state == sessionStateWaitingForQuit && cmd.commandCode != quitCmd {
			s.Out(mail.Codes.FailBadSequence)
			s.badCommandsCount++
			continue
		}
		s.log.Debug(cmd.String())
		// select the right handler by commandCode
		handler := handlers[cmd.commandCode]
		handler(s, cmd)
		if s.state == sessionStateAborted {
			break
		}
		// TODO is this legal?
		if s.badCommandsCount == s.srv.limits.BadCmds {
			s.Out(mail.Codes.FailMaxUnrecognizedCmd)
			s.state = sessionStateAborted
			break
		}
		// TODO timeout might differ as per https://tools.ietf.org/html/rfc5321#section-4.5.3.2
		s.conn.SetReadDeadline(time.Now().Add(s.srv.limits.CmdInput))
	}
	if err := s.bufin.Err(); err != nil {
		s.log.Error("bufin err", zap.Error(err))
	}
}

// send Welcome upon new session creation
func (s *session) handleWelcome() {
	cfg := s.srv.config
	smtpVersion := "SMTP"
	if cfg.TLSEnabled {
		smtpVersion = "ESMTP"
	}
	s.Out(fmt.Sprintf("220 %s %s %s(%s) %s", cfg.LocalName, smtpVersion, cfg.SftName, cfg.SftVersion, cfg.Announce))
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
	s.helloHost = cmd.Args()[0]
	// TODO check sending host (SPF)

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
	if s.srv.config.TLSEnabled { // do tls for this server
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
	if len(s.srv.authMechanisms) != 0 && s.srv.config.TLSEnabled {
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
	s.helloHost = cmd.Args()[0]
	// TODO check sending host (SPF)

	s.Out(fmt.Sprintf("250 %v hello %v", "mail.example", s.conn.RemoteAddr()))
}

// start TLS
func handleStartTLS(s *session, cmd *command) {
	// already started TLS
	if s.tls {
		s.state = sessionStateAborted
		s.Out(mail.Codes.FailBadSequence)
		return
	}

	s.Out(mail.Codes.SuccessStartTLSCmd)

	// set timeout for TLS connection negotiation
	s.conn.SetDeadline(time.Now().Add(DefaultLimits.TLSSetup))
	secureConn := tls.Server(s.conn, &tls.Config{Certificates: s.srv.config.Certificates})

	// TLS handshake
	if err := secureConn.Handshake(); err != nil {
		s.log.Info("start tls err", zap.Error(err))
		s.state = sessionStateAborted
		return
	}

	// reset session
	s.Reset()
	s.conn = secureConn
	s.bufin = bufio.NewScanner(secureConn)
	s.tls = true
	s.tlsState = secureConn.ConnectionState()
	s.state = sessionStateInit
}

func handleMail(s *session, cmd *command) {
	/*
		This command tells the SMTP-receiver that a new mail transaction is
		starting and to reset all its state tables and buffers, including any
		recipients or mail data.
	*/
	s.Reset()

	// require authentication if set in settings
	if len(s.srv.authMechanisms) != 0 && !s.authenticated {
		s.Out(mail.Codes.FailAccessDenied)
		return
	}

	// nested mail command
	if s.envelope.IsSet() {
		s.Out(mail.Codes.FailNestedMailCmd)
		return
	}

	args := cmd.Args()
	if len(args) == 0 {
		s.log.Debug("Empty arguments for MAIL cmd")
		s.Out(mail.Codes.FailInvalidAddress)
		return
	}

	// to lower and check if start with from: and is not empty
	from := strings.ToLower(strings.TrimSpace(args[0]))
	if from == "" || !strings.HasPrefix(from, "from:") {
		s.log.Debug("Invalid address for MAIL cmd")
		s.Out(mail.Codes.FailInvalidAddress)
		return
	}

	fromParts := strings.Split(from, ":")
	if len(fromParts) < 2 {
		s.Out(mail.Codes.FailInvalidAddress)
		return
	}
	s.envelope.MailFrom = mail.Address(removeBrackets(fromParts[1]))
	args = args[1:]

	// extensions size
	if len(args) > 0 {
		for _, ext := range args {
			extValue := strings.Split(ext, "=")
			if len(extValue) != 2 {
				s.Out(mail.Codes.FailInvalidAddress)
				return
			}
			switch strings.ToUpper(extValue[0]) {
			case "SIZE":
				size, err := strconv.ParseInt(extValue[1], 10, 64)
				if err != nil {
					s.Out(mail.Codes.FailInvalidExtension)
					return
				}
				if int64(size) > DefaultLimits.MsgSize {
					s.Out(mail.Codes.FailTooBig)
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
				s.envelope.MailFrom = ""
			}
		}
	}

	// validate email
	if err := s.envelope.MailFrom.Validate(); err != nil {
		s.Out(err.Error())
		return
	}

	// validate FQN
	if err := s.envelope.MailFrom.IsFQN(); err != "" {
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
		s.Out(mail.Codes.FailBadSequence)
		return
	}
	s.Out(mail.Codes.SuccessMailCmd)
}

func handleRcpt(s *session, cmd *command) {
	// if auth is required
	if len(s.srv.authMechanisms) != 0 && !s.authenticated {
		s.Out(mail.Codes.FailAccessDenied)
		return
	}

	// HELO/EHLO needs to be first
	if !s.helloSeen {
		s.Out(mail.Codes.FailBadSequence)
		return
	}

	// check recipients limit
	if len(s.envelope.MailTo) > DefaultLimits.MaxRcptCount {
		s.Out(mail.Codes.ErrorTooManyRecipients)
		return
	}

	args := cmd.Args()
	if args == nil {
		s.Out(mail.Codes.FailInvalidRecipient)
	}

	t := strings.Split(args[0], ":")
	if len(t) < 2 || strings.ToUpper(strings.TrimSpace(t[0])) != "TO" {
		s.Out(mail.Codes.FailInvalidAddress)
		return
	}

	/*
		TODO
		Servers MUST be prepared to encounter a list of source
		routes in the forward-path, but they SHOULD ignore the routes or MAY
		decline to support the relaying they imply.
	*/
	rcpt := mail.Address(t[1])

	// must be implemented - RFC5321
	if strings.ToLower(rcpt.Email()) == "postmaster" {
		rcpt = mail.Address("postmaster@" + s.srv.config.LocalName)
	}

	// validate email
	if err := rcpt.Validate(); err != nil {
		s.log.Debug("error validating address", zap.String("mail", string(rcpt)), zap.Error(err))
		s.Out(err.Error())
		return
	}

	// be relay for authenticated User
	if !s.authenticated {
		// check valid recipient if this email comes from outside
		err := s.srv.rcptValidator(rcpt)
		if err != nil {
			if err == ErrorRecipientNotFound {
				s.Out(mail.Codes.FailMailboxDoesntExist)
				return
			}
			if err == ErrorRecipientsMailboxFull {
				s.Out(mail.Codes.FailMailboxFull)
				return
			}
			s.Out(mail.Codes.FailAccessDenied)
			return
		}
	}

	// Add to recipients
	err := s.envelope.AddRecipient(rcpt)
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
		s.Out(mail.Codes.FailBadSequence)
		return
	}
	s.Out(mail.Codes.SuccessRcptCmd)
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
		s.Out(mail.Codes.FailBadSequence)
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
		s.Out(mail.Codes.SuccessDataCmd)
		s.state = sessionStateGettingData
	} else {
		s.Out(mail.Codes.FailBadSequence)
		s.state = sessionStateAborted
		return
	}

	// set data input time limit
	s.conn.SetReadDeadline(time.Now().Add(DefaultLimits.MsgInput))

	// TODO https://tools.ietf.org/html/rfc5321#section-4.5.3.1.6
	// read data, stop on EOF or reaching maximum sizes
	var size int64
	for s.bufin.Scan() && size < s.srv.limits.MsgSize {
		if s.bufin.Text() == "." {
			break
		}
		x := s.bufin.Bytes()
		size += int64(len(x))
		s.envelope.Write(x)
		s.envelope.Write([]byte("\r\n"))
	}
	// reading ended with error
	if err := s.bufin.Err(); err != nil {
		s.log.Error("bufin: ", zap.Error(err))
		s.Out(fmt.Sprintf(mail.Codes.FailReadErrorDataCmd, err))
		s.state = sessionStateAborted
		return
	}

	// reading ended by reaching maximum size
	if size > s.srv.limits.MsgSize {
		s.Out(mail.Codes.FailTooBig)
		return
	}

	// add received header
	/*
		When forwarding a message into or out of the Internet environment, a
		gateway MUST prepend a Received: line, but it MUST NOT alter in any
		way a Received: line that is already in the header section.
	*/
	s.envelope.Headers = s.ReceivedHeader()

	// add Message-ID, is user is aut
	if s.authenticated {
		s.envelope.Headers = append(s.envelope.Headers, []byte(fmt.Sprintf("Message-ID: <%d.%s@%s>\r\n", time.Now().Unix(), s.id, config.C.Me))...)
	}

	// data done
	s.state = sessionStateDataDone

	// add envelope to delivery system
	id, err := s.srv.mailHandler.Handle(s.envelope, s.authenticatedUser)
	if err != nil {
		s.Out("451 temporary queue error")
	} else {
		s.Out(fmt.Sprintf("%v %s", mail.Codes.SuccessMessageQueued, id))
	}

	// reset session
	s.DoneAndReset()
	return
}

// handleRset handle reset commands, reset currents session to beginning and empties the envelope
func handleRset(s *session, _ *command) {
	s.envelope.Reset()
	s.state = sessionStateInit
	s.Out(mail.Codes.SuccessResetCmd)
}

func handleNoop(s *session, _ *command) {
	s.Out(mail.Codes.SuccessNoopCmd)
}

func handleQuit(s *session, _ *command) {
	s.Out(mail.Codes.SuccessQuitCmd)
	s.state = sessionStateAborted
	s.log.Info("quit remote", zap.String("addr", s.conn.RemoteAddr().String()), zap.Duration("in", time.Since(s.start)))
}

func handleHelp(s *session, _ *command) {
	/*
		https://tools.ietf.org/html/rfc821
		This command causes the receiver to send helpful information
		to the sender of the HELP command.  The command may take an
		argument (e.g., any command name) and return more specific
		information as a response.
	*/
	s.Out(mail.Codes.SuccessHelpCmd + " CaN yOu HelP Me PLeasE!")
}
func handleBdat(s *session, cmd *command) {
	args := cmd.Args()

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
		s.Out(mail.Codes.FailUnrecognizedCmd) // TODO use the right code
		return
	}

	chunkSize64, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		s.Out(mail.Codes.FailUnrecognizedCmd) // TODO use the right code
		s.badCommandsCount++
		return
	}

	if (len(args) > 1 && strings.ToUpper(args[1]) == "LAST") || chunkSize64 == 0 {
		last = true
	}

	/*
		The message data is sent immediately after the trailing <CR>
		<LF> of the BDAT command line.  Once the receiver-SMTP receives the
		specified number of octets, it will return a 250 reply code.
	*/
	lr := io.LimitReader(s.conn, chunkSize64)
	/*
		If a failure occurs after a BDAT command is
		received, the receiver-SMTP MUST accept and discard the associated
		message data before sending the appropriate 5XX or 4XX code.
	*/
	if n, err := io.Copy(s.envelope.Data, lr); err != nil {
		s.Out(fmt.Sprintf(mail.Codes.FailReadErrorDataCmd, err))
		s.state = sessionStateAborted
		return
	} else if n != chunkSize64 {
		s.Out(fmt.Sprintf(mail.Codes.FailReadErrorDataCmd, err))
		s.state = sessionStateAborted
		return
	}

	if last {
		// data done
		s.Out("250 Message ok, TOTAL octets received") // TODO
		s.state = sessionStateDataDone
	} else {
		/*
			A 250 response MUST be sent to each successful BDAT data block within
			a mail transaction.
		*/
		s.Out(fmt.Sprintf("250 %d octets received", chunkSize64))
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
	// should not happen, some auth is always allowed
	if len(s.srv.authMechanisms) == 0 {
		s.Out(mail.Codes.FailCmdNotSupported)
		// AUTH with no AUTH enabled counts as a
		// bad command. This deals with a few people
		// who spam AUTH requests at non-supporting
		// servers.
		s.badCommandsCount++
		return
	}
	// if authenticatedUser is already
	if s.authenticated {
		// RFC4954, section 4: After an AUTH
		// command has been successfully
		// completed, no more AUTH commands
		// may be issued in the same session.
		s.Out("503 Out of sequence command")
		return
	}

	args := cmd.Args()
	if len(args) == 0 {
		s.Out("501 malformed auth input (#5.5.4)")
		s.log.Info("malformed auth input:", zap.String("cmd", cmd.String()))
		s.state = sessionStateAborted
		return
	}
	if !s.authMechanismValid(strings.ToUpper(args[0])) {
		s.Out(mail.Codes.ErrorCmdParamNotImplemented)
		return
	}

	switch strings.ToUpper(args[0]) {
	case "PLAIN":
		s.handlePlainAuth(cmd)
	case "LOGIN":
		s.handleLoginAuth(cmd)
	default:
		s.Out(mail.Codes.ErrorCmdParamNotImplemented)
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
	if len(s.authenticatedUser) != 0 {
		receivedHeader.WriteString(" authenticated as ")
		receivedHeader.WriteString(s.authenticatedUser)
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
	receivedHeader.WriteString(s.srv.config.SftName)
	receivedHeader.WriteByte(' ')
	receivedHeader.WriteString(s.srv.config.SftVersion)

	receivedHeader.WriteString("; id ")
	receivedHeader.WriteString(s.id)

	// timestamp
	receivedHeader.WriteString("; ")
	receivedHeader.WriteString(time.Now().Format(time.RFC1123))
	receivedHeader.WriteString("\r\n")

	header := receivedHeader.Bytes()

	// fold header
	mail.FoldHeader(&header)
	return header
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
