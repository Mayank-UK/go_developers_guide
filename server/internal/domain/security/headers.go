package security

// HeaderRule is one security response header: the exact value the
// middleware sets and the attack the header closes — the Why rides
// beside the rule so the demo can teach from the same source of truth
// the middleware applies.
type HeaderRule struct {
	Header string
	Value  string
	Why    string
}

// APISecurityHeaders is the guide's header policy for JSON API
// responses. It is deliberately the STRICT set an API can carry: an
// API renders no first-party content, so its CSP can refuse
// everything, where an HTML front end would scope its policy to the
// scripts and styles it actually loads. HSTS is stated even though the
// dev stack is plain http — browsers ignore the header over http, so
// it is inert in dev and correct the day TLS terminates in front of
// the service.
var APISecurityHeaders = []HeaderRule{
	{
		Header: "Content-Security-Policy",
		Value:  "default-src 'none'; frame-ancestors 'none'",
		Why:    "an API response renders nothing: refuse every source and every framing, so a content-sniff detour has nothing to execute and the response cannot be framed",
	},
	{
		Header: "Strict-Transport-Security",
		Value:  "max-age=31536000; includeSubDomains",
		Why:    "once a client has seen the site over https, it refuses plain http for a year, subdomains included — a stripped-cookie replay over http dies here",
	},
	{
		Header: "X-Content-Type-Options",
		Value:  "nosniff",
		Why:    "the browser must not second-guess application/json as something executable — content-type sniffing is how a JSON error body becomes a script",
	},
	{
		Header: "X-Frame-Options",
		Value:  "DENY",
		Why:    "no page may embed this response in a frame — clickjacking puts your real buttons under an attacker's overlay",
	},
	{
		Header: "Referrer-Policy",
		Value:  "no-referrer",
		Why:    "the full URL — query strings included — must not travel to whatever the client visits next; URLs carry more PII than most logs",
	},
}
