package mail

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type testAddress struct {
	addr  Address
	str   string
	host  string
	user  string
	real  bool
	valid bool
}

var ta = []testAddress{
	{
		// real and valid
		addr:  Address("dzivjak@matous.me"),
		str:   "dzivjak@matous.me",
		host:  "matous.me",
		user:  "dzivjak",
		valid: true,
		real:  true,
	},
	{
		// not real but valid
		addr:  Address("definitelynotarealaddress@notarealdomaintoo.com"),
		str:   "definitelynotarealaddress@notarealdomaintoo.com",
		host:  "notarealdomaintoo.com",
		user:  "definitelynotarealaddress",
		valid: true,
		real:  false,
	},
	{
		// too long user name
		addr:  Address("definitelynotarealaddressandalsotoolongtoberealaaaaaaaaaaaaaaaaaa@notarealdomaintoo.com"),
		str:   "definitelynotarealaddressandalsotoolongtoberealaaaaaaaaaaaaaaaaaa@notarealdomaintoo.com",
		host:  "notarealdomaintoo.com",
		user:  "definitelynotarealaddressandalsotoolongtoberealaaaaaaaaaaaaaaaaaa",
		valid: false,
		real:  false,
	},
}

func TestAddress_Email(t *testing.T) {
	for _, a := range ta {
		assert.Equal(t, a.addr.Email(), a.str)
	}
}

func TestAddress_Hostname(t *testing.T) {
	for _, a := range ta {
		assert.Equal(t, a.addr.Hostname(), a.host)
	}
}

func TestAddress_User(t *testing.T) {
	for _, a := range ta {
		assert.Equal(t, a.addr.User(), a.user)
	}
}

func TestAddress_Validate(t *testing.T) {
	for _, a := range ta {
		if a.valid {
			assert.Equal(t, a.addr.Validate(), "", "address marked as invalid although it's valid")
		} else {
			assert.NotEqual(t, a.addr.Validate(), "", "address marked as valid although it's not valid")
		}
	}
}

func TestAddress_IsFQN(t *testing.T) {
	for _, a := range ta {
		if a.real {
			assert.Equal(t, a.addr.IsFQN(), "", "address marked as not real although it's real")
		} else {
			assert.NotEqual(t, a.addr.IsFQN(), "", "address marked as real although it's not real")
		}
	}
}
