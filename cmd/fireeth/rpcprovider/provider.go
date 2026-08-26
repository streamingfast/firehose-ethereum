// Package rpcprovider parses the poller's RPC provider definitions, coming either from the
// legacy positional <rpc-endpoint> argument or from the repeatable --provider flag, together
// with the --headers flag which can be either global or scoped to a single provider.
package rpcprovider

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// DefaultProviderName is the name given to the provider defined through the positional
// <rpc-endpoint> argument, which is always the highest priority provider.
const DefaultProviderName = "default"

// redacted is what replaces the parts of an endpoint URL that are likely to hold a credential.
const redacted = "redacted"

var errEndpointNoHost = errors.New(`endpoint URL has no host, it must be an absolute URL like "https://host/path"`)

// Provider is a single RPC endpoint the poller fetches blocks from. The order of a []Provider
// is the priority order: the first one is tried first.
type Provider struct {
	// Name identifies the provider in logs and errors, it is unique across all providers.
	Name string
	// URL is the raw endpoint URL, credentials included, use [Provider.RedactedURL] when logging it.
	URL string
	// Headers are the HTTP headers to send with each request to this provider, in flag order.
	Headers []Header
}

// Header is a single HTTP header to send to a provider.
type Header struct {
	Key   string
	Value string
}

// RedactedURL returns the provider's URL with everything that could hold a credential removed,
// keeping only the scheme and the host. It is what should be printed in logs, API keys are
// commonly embedded in the path (`https://host/v1/<api-key>`) or in the query string.
func (p Provider) RedactedURL() string {
	u, err := url.Parse(p.URL)
	if err != nil {
		return "(unparsable endpoint URL)"
	}

	if u.User != nil {
		u.User = url.User(redacted)
	}

	if u.Path != "" && u.Path != "/" {
		u.Path = "/" + redacted
		u.RawPath = ""
	}

	if u.RawQuery != "" {
		u.RawQuery = redacted
	}

	u.Fragment = ""
	u.RawFragment = ""

	return u.String()
}

// Parse turns the poller's endpoint inputs into an ordered list of providers.
//
// The positionalEndpoint, when non-empty, always becomes the first (highest priority) provider
// and is named [DefaultProviderName]. Each providerFlags entry is `<url>[#<name>]`, defaulting
// the name to the URL's host, and each headerFlags entry is `<key>: <value>[#<provider-name>]`,
// applying to every provider when it carries no provider name.
//
// Both `#` separators are found by splitting on the **last** `#` of the flag's value.
func Parse(positionalEndpoint string, providerFlags []string, headerFlags []string) ([]Provider, error) {
	providers, err := parseProviders(positionalEndpoint, providerFlags)
	if err != nil {
		return nil, err
	}

	if err := applyHeaders(providers, headerFlags); err != nil {
		return nil, err
	}

	return providers, nil
}

func parseProviders(positionalEndpoint string, providerFlags []string) ([]Provider, error) {
	var providers []Provider

	if positionalEndpoint != "" {
		if _, err := parseEndpoint(positionalEndpoint); err != nil {
			return nil, fmt.Errorf("invalid <rpc-endpoint> argument %q: %w", positionalEndpoint, err)
		}

		providers = append(providers, Provider{Name: DefaultProviderName, URL: positionalEndpoint})
	}

	for _, flag := range providerFlags {
		endpoint, name, named := splitOnLastHash(flag)
		if endpoint == "" {
			return nil, fmt.Errorf("invalid --provider %q: empty endpoint URL", flag)
		}

		if named && name == "" {
			return nil, fmt.Errorf("invalid --provider %q: empty provider name after '#'", flag)
		}

		endpointURL, err := parseEndpoint(endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid --provider %q: %w", flag, err)
		}

		if !named {
			name = endpointURL.Host
		}

		if slices.ContainsFunc(providers, func(p Provider) bool { return p.Name == name }) {
			return nil, fmt.Errorf("invalid --provider %q: provider name %q is already used by another provider", flag, name)
		}

		providers = append(providers, Provider{Name: name, URL: endpoint})
	}

	if len(providers) == 0 {
		return nil, errors.New("no RPC provider defined, provide one as the <rpc-endpoint> positional argument or through at least one --provider flag")
	}

	return providers, nil
}

func applyHeaders(providers []Provider, headerFlags []string) error {
	for _, flag := range headerFlags {
		definition, providerName, scoped := splitOnLastHash(flag)
		if scoped && providerName == "" {
			return fmt.Errorf("invalid --headers %q: empty provider name after '#'", flag)
		}

		key, value, found := strings.Cut(definition, ":")
		if !found {
			return fmt.Errorf(`invalid --headers %q: expected format "Key: Value" or "Key: Value#<provider-name>"`, flag)
		}

		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("invalid --headers %q: empty header key", flag)
		}

		header := Header{Key: key, Value: value}

		if !scoped {
			for i := range providers {
				providers[i].Headers = append(providers[i].Headers, header)
			}
			continue
		}

		index := slices.IndexFunc(providers, func(p Provider) bool { return p.Name == providerName })
		if index < 0 {
			return fmt.Errorf("invalid --headers %q: no provider named %q, known providers are %v", flag, providerName, Names(providers))
		}

		providers[index].Headers = append(providers[index].Headers, header)
	}

	return nil
}

// Names returns the provider names, in priority order.
func Names(providers []Provider) []string {
	names := make([]string, len(providers))
	for i, provider := range providers {
		names[i] = provider.Name
	}

	return names
}

func parseEndpoint(endpoint string) (*url.URL, error) {
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("unable to parse endpoint URL: %w", err)
	}

	if endpointURL.Host == "" {
		return nil, errEndpointNoHost
	}

	return endpointURL, nil
}

// splitOnLastHash splits `value#suffix` on its **last** '#'. A value legitimately ending with
// `#something` is going to be mis-split, this is a known and accepted limitation, there is no
// escaping syntax.
func splitOnLastHash(in string) (value string, suffix string, found bool) {
	index := strings.LastIndex(in, "#")
	if index < 0 {
		return in, "", false
	}

	return in[:index], in[index+1:], true
}
