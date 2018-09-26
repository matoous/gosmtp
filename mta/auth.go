package mta

import (
	"encoding/base64"
	"gosmtp/mail"
	"strings"

	"go.uber.org/zap"
)

/*
SupportedAuthMechanisms is array of string describing currently supported/implemented
authentication mechanisms
*/
var SupportedAuthMechanisms = []string{"LOGIN", "PLAIN"}

func (s *session) handleLoginAuth(cmd *command) {
	args := cmd.Args()
	var username string
	var password []byte
	if len(args) == 1 {
		s.Out("334 VXNlcm5hbWU6")
		s.bufin.Scan()
		cmd := strings.TrimSpace(s.bufin.Text())
		s.log.Debug(cmd)
		data, err := base64.StdEncoding.DecodeString(cmd)
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Info("malformed auth input", zap.String("cmd", cmd))
			s.state = sessionStateAborted
			return
		}
		username = string(data)
		s.Out("334 UGFzc3dvcmQ6")
		s.bufin.Scan()
		cmd = strings.TrimSpace(s.bufin.Text())
		s.log.Debug(cmd)
		password, err = base64.StdEncoding.DecodeString(cmd)
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Info("malformed auth input", zap.String("cmd", cmd))
			s.state = sessionStateAborted
			return
		}
	} else if len(args) == 2 {
		data, err := base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Info("malformed auth input", zap.String("cmd", cmd.data))
			s.state = sessionStateAborted
			return
		}
		username = string(data)
		s.Out("334 UGFzc3dvcmQ6")
		s.bufin.Scan()
		cmd := s.bufin.Text()
		s.log.Debug(cmd)
		password, err = base64.StdEncoding.DecodeString(cmd)
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Info("malformed auth input", zap.String("cmd", cmd))
			s.state = sessionStateAborted
			return
		}
	}

	// check login
	// check login
	ok, err := s.srv.authFunc(username, password)
	if err != nil {
		s.Out(mail.Codes.ErrorAuth)
		s.state = sessionStateAborted
		return
	}
	if !ok {
		s.Out(mail.Codes.FailAuthentication)
		s.state = sessionStateAborted
		return
	}
	s.authenticatedUser = username

	// login succeeded
	s.authenticated = true
	s.Out(mail.Codes.SuccessAuthentication)
}

func (s *session) handlePlainAuth(cmd *command) {
	args := cmd.Args()
	var authData []byte
	var err error
	// check that PLAIN auth input is of valid form
	if len(args) == 1 {
		s.Out("334")
		s.bufin.Scan()
		cmd, err := parseCommand(s.bufin.Text())
		s.log.Debug(cmd.data)
		if err != nil {
			s.Out(mail.Codes.FailUnrecognizedCmd)
			s.badCommandsCount++
		}
		authData, err = base64.StdEncoding.DecodeString(cmd.Args()[0])
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Info("malformed auth input", zap.String("cmd", cmd.data))
			s.state = sessionStateAborted
			return
		}
	} else if len(args) == 2 {
		authData, err = base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Info("malformed auth input", zap.String("cmd", cmd.data))
			s.state = sessionStateAborted
			return
		}
	}

	// split
	t := make([][]byte, 3)
	i := 0
	for _, b := range authData {
		if b == 0 {
			i++
			continue
		}
		t[i] = append(t[i], b)
	}
	//authId := string(t[0])
	authLogin := string(t[1])
	authPasswd := t[2]

	// check login
	ok, err := s.srv.authFunc(authLogin, authPasswd)
	if err != nil {
		s.Out(mail.Codes.ErrorAuth)
		s.state = sessionStateAborted
		return
	}
	if !ok {
		s.Out(mail.Codes.FailAuthentication)
		s.state = sessionStateAborted
		return
	}
	s.authenticatedUser = authLogin

	// login succeeded
	s.authenticated = true
	s.Out(mail.Codes.SuccessAuthentication)
}
