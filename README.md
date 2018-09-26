# GOSMTP

GOSMTP is golang implementation of full-featured, RFC standard compliant and lightweight SMTP server.

## Mail Transport Agent

Accepts and handles incoming mail.
Supported authentication methods:

* PLAIN
* LOGIN

## Setup

### Download

1. Download

2. Clone

### Config

Minimal configuration you will need might look like this:

```toml
me: example.com
```

With this minimal configuration you will be able to run your email server at given
domain. This server will use defaults for all you haven't specified.

### Run

To start the server just simply run

```go
./mamail
```

## About

### TLS

GOSMTP aims to be secure and modern. TLS should be always allowed as specified in

> As a result, clients and servers SHOULD implement both STARTTLS on
> port 587 and Implicit TLS on port 465 for this transition period.
> Note that there is no significant difference between the security
> properties of STARTTLS on port 587 and Implicit TLS on port 465 if
> the implementations are correct and if both the client and the server
> are configured to require successful negotiation of TLS prior to
> Message Submission.

Fot testing purpouses one can generate certificate via [https://github.com/deckarep/EasyCert](https://github.com/deckarep/EasyCert)

### DNS

#### MX Records

It is recommended that MSPs advertise MX records for the handling of
inbound mail (instead of relying entirely on A or AAAA records) and
that those MX records be signed using DNSSEC [RFC4033].  This is
mentioned here only for completeness, as the handling of inbound mail
is out of scope for this document.

#### SRV Records

MSPs SHOULD advertise SRV records to aid MUAs in determining the
proper configuration of servers, per the instructions in [RFC6186].

MSPs SHOULD advertise servers that support Implicit TLS in preference
to servers that support cleartext and/or STARTTLS operation.

#### DNSSEC

All DNS records advertised by an MSP as a means of aiding clients in
communicating with the MSP's servers SHOULD be signed using DNSSEC if
and when the parent DNS zone supports doing so.

#### TLSA Records

MSPs SHOULD advertise TLSA records to provide an additional trust
anchor for public keys used in TLS server certificates.  However,
TLSA records MUST NOT be advertised unless they are signed using
DNSSEC.

### SPF

Publishing Authorization

An SPF-compliant domain MUST publish a valid SPF record as described
in Section 3.  This record authorizes the use of the domain name in
the "HELO" and "MAIL FROM" identities by the MTAs it specifies.

If domain owners choose to publish SPF records, it is RECOMMENDED
that they end in "-all", or redirect to other records that do, so
that a definitive determination of authorization can be made.

Domain holders may publish SPF records that explicitly authorize no
hosts if mail should never originate using that domain.

When changing SPF records, care must be taken to ensure that there is
a transition period so that the old policy remains valid until all
legitimate E-Mail has been checked.

## Contributing

### Goals

* full-featured, production ready MTA agent
* easily extensible