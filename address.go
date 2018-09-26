package gosmtp

import (
	"bytes"
	"fmt"
	"net"
	"net/mail"
	"strings"
)

func parseAddress(src string) (*mail.Address, error) {
	// While a RFC5321 mailbox specification is not the same as an RFC5322
	// email address specification, it is better to accept that format and
	// parse it down to the actual address, as there are a lot of badly
	// behaving MTAs and MUAs that do it wrongly. It therefore makes sense
	// to rely on Go's built-in address parser. This does have the benefit
	// of allowing "email@example.com" as input as thats commonly used,
	// though not RFC compliant.
	addr, err := mail.ParseAddress(src)
	if err != nil {
		return nil, fmt.Errorf("malformed e-mail address: %s", src)
	}
	return addr, nil
}

func hostname(addr *mail.Address) string {
	// if the mail address didn't have @ it would be caought by parseAddress
	return string(bytes.Split([]byte(addr.Address), []byte{'@'})[1])
}

// IsFQN checks if email host is full qualified name (MX or A record)
func IsFQN(addr *mail.Address) string {
	ok, err := fqn(hostname(addr))
	if err != nil {
		return Codes.ErrorUnableToResolveHost
	} else if !ok {
		return Codes.FailUnqalifiedHostName
	}
	return ""
}

// isFQN checks if domain is FQN (MX or A record) and caches the result
// TODO refactor
func fqn(host string) (bool, error) {
	_, err := net.LookupMX(host)
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such host") {
			_, err = net.LookupHost(host)
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
