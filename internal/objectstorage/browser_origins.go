package objectstorage

import (
	"net"
	"net/url"
	"regexp"
	"strings"
)

// BrowserOrigins returns exact provider origins for direct upload/playback CSP.
// It never contains credentials, signed query parameters, paths or wildcards.
func BrowserOrigins(c Config) []string {
	if c.Driver != "s3" {
		return nil
	}
	u, e := url.Parse(c.Endpoint)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(u.Host, "*; \t\r\n") {
		return nil
	}
	result := []string{u.Scheme + "://" + u.Host}
	if !c.PathStyle && net.ParseIP(u.Hostname()) == nil && regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`).MatchString(c.Bucket) {
		result = append(result, u.Scheme+"://"+c.Bucket+"."+u.Host)
	}
	return result
}
