package mta

import (
	"errors"
	"strings"
)

type command struct {
	commandCode int
	verb        string
	data        string
}

const (
	heloCmd = iota
	ehloCmd
	quitCmd
	rsetCmd
	noopCmd
	mailCmd
	rcptCmd
	dataCmd
	starttlsCmd
	vrfyCmd
	expnCmd
	helpCmd
	authCmd
	bdatCmd
)

/*
isall7bit returns true if the argument is all 7-bit ASCII. This is what all SMTP
commands are supposed to be, and later things are going to screw up if
some joker hands us UTF-8 or any other equivalent.
*/
func isall7bit(b []byte) bool {
	for _, c := range b {
		if c > 127 {
			return false
		}
	}
	return true
}

/*
parseCommand parses command from string or returns error if it is not possible
or the command doesn't exist
*/
func parseCommand(line string) (*command, error) {
	parts := strings.SplitN(line, " ", 2)

	if len(parts) == 0 {
		return nil, errors.New("command empty")
	}

	// Check that command doesn't contain UTF-8 and other smelly stuff
	if !isall7bit([]byte(parts[0])) {
		return nil, errors.New("command contains non 7-bit ASCII")
	}

	// Search if the command is in our command list
	var cmdCode = -1
	switch strings.ToUpper(parts[0]) {
	case "HELO":
		cmdCode = heloCmd
	case "EHLO":
		cmdCode = ehloCmd
	case "QUIT":
		cmdCode = quitCmd
	case "RSET":
		cmdCode = rsetCmd
	case "NOOP":
		cmdCode = noopCmd
	case "MAIL":
		cmdCode = mailCmd
	case "RCPT":
		cmdCode = rcptCmd
	case "DATA":
		cmdCode = dataCmd
	case "STARTTLS":
		cmdCode = starttlsCmd
	case "VRFY":
		cmdCode = vrfyCmd
	case "EXPN":
		cmdCode = expnCmd
	case "HELP":
		cmdCode = helpCmd
	case "AUTH":
		cmdCode = authCmd
	case "BDAT":
		cmdCode = bdatCmd
	default:
		return nil, errors.New("unrecognized command")
	}

	cmd := &command{
		cmdCode,
		parts[0],
		"",
	}

	if len(parts) > 1 {
		cmd.data = parts[1]
	}

	// Check for verbs defined not to have an argument
	// (RFC 5321 s4.1.1)
	switch cmd.commandCode {
	case rsetCmd, dataCmd, quitCmd:
		if cmd.data != "" {
			return nil, errors.New("unexpected argument")
		}
	}
	return cmd, nil
}

/*
String returns back the original line with command as a string
*/
func (cmd *command) String() string {
	if cmd.data != "" {
		return cmd.verb + " " + cmd.data
	}
	return cmd.verb
}

/*
Args returns array of strings with individual command arguments
*/
func (cmd *command) Args() []string {
	if cmd.data == "" {
		return nil
	}
	return strings.Split(cmd.data, " ")
}
