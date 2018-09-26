package mail

import (
	"errors"
	"net"
	"regexp"
	"strings"
)

type Address string

// email address size limits
const (
	maxEmailLength      = 256 // max full email address length
	maxLocalPartLength  = 64  // max length of user/local part of email address as per https://tools.ietf.org/html/rfc5321#section-4.5.3.1.1
	maxDomainPartLength = 255 // max length of domain part of email address https://tools.ietf.org/html/rfc5321#section-4.5.3.1.2
)

// simple regex to check email format validity
var emailRegexp = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// TODO move to codes
var (
	FailReversePathTooLong = "550 reverse path must be lower than 255 char (RFC 5321 4.5.1.3.1)"
	FailLocalPartTooLong   = "550 local part of reverse path MUST be lower than 65 char (RFC 5321 4.5.3.1.1)"
	FailDomainPartTooLong  = "550 domain part of reverse path MUST be lower than 256 char (RFC 5321 4.5.3.1.1)"
)

// ValidFormat checks if email is of valid format
func (a Address) ValidFormat() bool {
	if len(strings.Split(a.Email(), "@")) != 2 {
		return false
	}
	return true
}

// Email returns whole email address
func (a Address) Email() string {
	return a.User() + "@" + a.Hostname()
}

// Hostname returns the name of the host of email address
func (a Address) Hostname() string {
	e := string(a)
	if idx := strings.Index(e, "@"); idx != -1 {
		return strings.ToLower(e[idx+1:])
	}
	return ""
}

// Validate validates given email address
// checks size and format
func (a Address) Validate() error {
	// 0 len -> bounce
	if len(a) > 0 {
		if !a.ValidFormat() {
			return errors.New(Codes.FailInvalidAddress)
		}
		if len(a) > maxEmailLength {
			return errors.New(FailReversePathTooLong)
		}
		if len(a.User()) > maxLocalPartLength {
			return errors.New(FailLocalPartTooLong)
		}
		if len(a.Hostname()) > maxDomainPartLength {
			return errors.New(FailDomainPartTooLong)
		}
	}
	return nil
}

// IsFQN checks if email host is full qualified name (MX or A record)
func (a Address) IsFQN() string {
	if len(a) > 0 {
		ok, err := fqn(a.Hostname())
		if err != nil {
			return Codes.ErrorUnableToResolveHost
		} else if !ok {
			return Codes.FailUnqalifiedHostName
		}
	}
	return ""
}

// User returns user part of email address
func (a Address) User() string {
	e := string(a)
	if idx := strings.Index(e, "@"); idx != -1 {
		return strings.ToLower(e[:idx])
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
