package gosmtp

import (
	"encoding/base64"
	"strings"
)

/*
SupportedAuthMechanisms is array of string describing currently supported/implemented
authentication mechanisms
*/
var SupportedAuthMechanisms = []string{"LOGIN", "PLAIN"}

func (s *session) handleLoginAuth(cmd *command) {
	args := cmd.arguments
	var username string
	var password []byte
	if len(args) == 1 {
		s.Out("334 VXNlcm5hbWU6")
		s.bufin.Scan()
		cmd := strings.TrimSpace(s.bufin.Text())
		s.log.Printf("INFO: received AUTH cmd: '%s'", cmd)
		data, err := base64.StdEncoding.DecodeString(cmd)
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Printf("DEBUG: malformed auth input: '%s'", cmd)
			s.state = sessionStateAborted
			return
		}
		username = string(data)
		s.Out("334 UGFzc3dvcmQ6")
		s.bufin.Scan()
		cmd = strings.TrimSpace(s.bufin.Text())
		s.log.Printf("INFO: received second AUTH cmd: '%s'", cmd)
		password, err = base64.StdEncoding.DecodeString(cmd)
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Printf("DEBUG: malformed auth input: '%s'", cmd)
			s.state = sessionStateAborted
			return
		}
	} else if len(args) == 2 {
		data, err := base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Printf("DEBUG: malformed auth input: '%s'", cmd)
			s.state = sessionStateAborted
			return
		}
		username = string(data)
		s.Out("334 UGFzc3dvcmQ6")
		s.bufin.Scan()
		cmd := s.bufin.Text()
		s.log.Printf("INFO: received second AUTH cmd: '%s'", cmd)
		password, err = base64.StdEncoding.DecodeString(cmd)
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Printf("DEBUG: malformed auth input: '%s'", cmd)
			s.state = sessionStateAborted
			return
		}
	}

	// check login
	// check login
	s.peer.Username = username
	ok, err := s.srv.Authenticator(s.peer, password)
	if err != nil {
		s.Out(Codes.ErrorAuth)
		s.state = sessionStateAborted
		return
	}
	if !ok {
		s.Out(Codes.FailAuthentication)
		s.state = sessionStateAborted
		return
	}

	// login succeeded
	s.peer.Authenticated = true
	s.Out(Codes.SuccessAuthentication)
}

func (s *session) handlePlainAuth(cmd *command) {
	args := cmd.arguments
	var authData []byte
	var err error
	// check that PLAIN auth input is of valid form
	if len(args) == 1 {
		s.Out("334")
		s.bufin.Scan()
		cmd, err := parseCommand(s.bufin.Text())
		s.log.Printf("INFO: received AUTH cmd: '%s'", cmd)
		if err != nil {
			s.Out(Codes.FailUnrecognizedCmd)
			s.badCommandsCount++
		}
		authData, err = base64.StdEncoding.DecodeString(cmd.arguments[0])
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Printf("DEBUG: malformed auth input: '%s'", cmd)
			s.state = sessionStateAborted
			return
		}
	} else if len(args) == 2 {
		authData, err = base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			s.Out("501 malformed auth input")
			s.log.Printf("DEBUG: malformed auth input: '%s'", cmd)
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
	s.peer.Username = authLogin
	ok, err := s.srv.Authenticator(s.peer, authPasswd)
	if err != nil {
		s.Out(Codes.ErrorAuth)
		s.state = sessionStateAborted
		return
	}
	if !ok {
		s.Out(Codes.FailAuthentication)
		s.state = sessionStateAborted
		return
	}
	s.peer.Authenticated = true

	// login succeeded
	s.Out(Codes.SuccessAuthentication)
}
