package cookies

import (
	"bytes"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/logger"
	requestutil "github.com/oauth2-proxy/oauth2-proxy/v7/pkg/requests/util"
	tenantutils "github.com/oauth2-proxy/oauth2-proxy/v7/pkg/tenant/utils"
)

// MakeCookieFromOptions constructs a cookie based on the given *options.CookieOptions,
// value and creation time
func MakeCookieFromOptions(req *http.Request, name string, value string, opts *options.Cookie, expiration time.Duration, now time.Time) *http.Cookie {

	domains := renderCookieDomainsTemplate(opts, req)
	domain := GetCookieDomain(req, domains)
	// If nothing matches, create the cookie with the shortest domain
	if domain == "" && len(opts.Domains) > 0 {
		logger.Errorf("Warning: request host %q did not match any of the specific cookie domains of %q",
			requestutil.GetRequestHost(req),
			strings.Join(opts.Domains, ","),
		)
		domain = opts.Domains[len(domains)-1]
	}

	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     opts.Path,
		Domain:   domain,
		Expires:  now.Add(expiration),
		HttpOnly: opts.HTTPOnly,
		Secure:   opts.Secure,
		SameSite: ParseSameSite(opts.SameSite),
	}

	warnInvalidDomain(c, req)

	return c
}

func renderCookieDomainsTemplate(opts *options.Cookie, req *http.Request) []string {
	domains := make([]string, len(opts.Domains))

	for i, d := range opts.Domains {
		t, err := template.New("cookiesDomain").Parse(d)
		if err != nil {
			logger.Errorf("unable to parse cookie domain Template")
		}
		var buf bytes.Buffer
		t.Execute(&buf, map[string]string{
			"TENANT_ID": tenantutils.FromContext(req.Context()),
		})
		domains[i] = buf.String()
	}
	return domains
}

// GetCookieDomain returns the correct cookie domain given a list of domains
// by checking the X-Fowarded-Host and host header of an an http request
func GetCookieDomain(req *http.Request, cookieDomains []string) string {
	host := requestutil.GetRequestHost(req)
	for _, domain := range cookieDomains {
		if strings.HasSuffix(host, domain) {
			return domain
		}
	}
	return ""
}

// Parse a valid http.SameSite value from a user supplied string for use of making cookies.
func ParseSameSite(v string) http.SameSite {
	switch v {
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "":
		return 0
	default:
		panic(fmt.Sprintf("Invalid value for SameSite: %s", v))
	}
}

// warnInvalidDomain logs a warning if the request host and cookie domain are
// mismatched.
func warnInvalidDomain(c *http.Cookie, req *http.Request) {
	if c.Domain == "" {
		return
	}

	host := requestutil.GetRequestHost(req)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if !strings.HasSuffix(host, c.Domain) {
		logger.Errorf("Warning: request host is %q but using configured cookie domain of %q", host, c.Domain)
	}
}
