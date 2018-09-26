package gosmtp

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/signalsciences/tlstext"
)

const (
	maxCommandLineLength = 512
)

func tlsVersionString(conn *tls.ConnectionState) string {
	return tlstext.VersionFromConnection(conn)
}

func tlsCiherSuiteString(conn *tls.ConnectionState) string {
	return tlstext.CipherSuiteFromConnection(conn)
}

func tlsInfo(conn *tls.ConnectionState) string {
	return fmt.Sprintf("(using %s with cipher %s)", tlsVersionString(conn), tlsCiherSuiteString(conn))
}

// removeBrackets removes trailing and ending brackets (<string> -> string)
func removeBrackets(s string) string {
	if strings.HasPrefix(s, "<") {
		s = s[1:]
	}
	if strings.HasSuffix(s, ">") {
		s = s[0 : len(s)-1]
	}
	return s
}

func limitedLineSplitter(data []byte, atEOF bool) (advance int, token []byte, err error) {
	dropCR := func(data []byte) []byte {
		if len(data) > 0 && data[len(data)-1] == '\r' {
			return data[0 : len(data)-1]
		}
		return data
	}
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 && i < maxCommandLineLength {
		return i + 1, dropCR(data[0:i]), nil
	} else if i >= 0 {
		l := min(len(data), maxCommandLineLength)
		return l + 1, dropCR(data[0:l]), nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	return 0, nil, nil
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// DummyMailStore for testing purpouses, doesn't handle the mail
type DummyMailStore struct{}

// Handle discards the email
func (dms *DummyMailStore) Handle(envelope *Envelope, user string) (id string, err error) {
	return "QUEUED_MAIL_ID", nil
}

// Wrap a byte slice paragraph for use in SMTP header
func wrap(sl []byte) []byte {
	length := 0
	for i := 0; i < len(sl); i++ {
		if length > 76 && sl[i] == ' ' {
			sl = append(sl, 0, 0)
			copy(sl[i+2:], sl[i:])
			sl[i] = '\r'
			sl[i+1] = '\n'
			sl[i+2] = '\t'
			i += 2
			length = 0
		}
		if sl[i] == '\n' {
			length = 0
		}
		length++
	}
	return sl
}
