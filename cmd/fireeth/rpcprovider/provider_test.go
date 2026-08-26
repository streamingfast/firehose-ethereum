package rpcprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_positionalEndpointOnly(t *testing.T) {
	providers, err := Parse("https://primary.example.com/v1/abcd", nil, nil)
	require.NoError(t, err)

	assert.Equal(t, []Provider{
		{Name: "default", URL: "https://primary.example.com/v1/abcd"},
	}, providers)
}

func TestParse_providerFlagsOnly(t *testing.T) {
	providers, err := Parse("", []string{
		"https://primary.example.com/v1/abcd#primary",
		"https://fallback.example.com/efgh#fallback",
	}, nil)
	require.NoError(t, err)

	assert.Equal(t, []Provider{
		{Name: "primary", URL: "https://primary.example.com/v1/abcd"},
		{Name: "fallback", URL: "https://fallback.example.com/efgh"},
	}, providers)
}

func TestParse_providerNameDefaultsToHost(t *testing.T) {
	providers, err := Parse("", []string{
		"https://primary.example.com/v1/abcd",
		"http://localhost:8545",
	}, nil)
	require.NoError(t, err)

	assert.Equal(t, []Provider{
		{Name: "primary.example.com", URL: "https://primary.example.com/v1/abcd"},
		{Name: "localhost:8545", URL: "http://localhost:8545"},
	}, providers)
}

// Some providers embed the API key in the middle of the path, followed by more path segments
// (Avalanche C-Chain style endpoints are a common example). The URL must go through untouched,
// trailing slash included, and the name must default to the host only.
func TestParse_apiKeyInTheMiddleOfThePath(t *testing.T) {
	const endpointURL = "https://node.avalanche-mainnet.example.com/6eebf479d18c8fa23ffad796d82c9f148aaac6f4/ext/bc/C/rpc/"

	providers, err := Parse(endpointURL, []string{endpointURL + "#avalanche"}, nil)
	require.NoError(t, err)

	assert.Equal(t, []Provider{
		{Name: "default", URL: endpointURL},
		{Name: "avalanche", URL: endpointURL},
	}, providers)

	providers, err = Parse("", []string{endpointURL}, nil)
	require.NoError(t, err)

	assert.Equal(t, []Provider{
		{Name: "node.avalanche-mainnet.example.com", URL: endpointURL},
	}, providers)
}

func TestParse_positionalIsAlwaysPriorityZero(t *testing.T) {
	providers, err := Parse("https://primary.example.com", []string{
		"https://a.example.com#a",
		"https://b.example.com#b",
	}, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"default", "a", "b"}, names(providers))
}

func TestParse_noProviderAtAll(t *testing.T) {
	_, err := Parse("", nil, nil)
	assert.EqualError(t, err, `no RPC provider defined, provide one as the <rpc-endpoint> positional argument or through at least one --provider flag`)
}

func TestParse_duplicateProviderNames(t *testing.T) {
	_, err := Parse("", []string{
		"https://a.example.com#same",
		"https://b.example.com#same",
	}, nil)
	assert.EqualError(t, err, `invalid --provider "https://b.example.com#same": provider name "same" is already used by another provider`)
}

func TestParse_duplicateProviderNamesAgainstPositional(t *testing.T) {
	_, err := Parse("https://a.example.com", []string{"https://b.example.com#default"}, nil)
	assert.EqualError(t, err, `invalid --provider "https://b.example.com#default": provider name "default" is already used by another provider`)
}

func TestParse_duplicateProviderNamesFromDefaultedHost(t *testing.T) {
	_, err := Parse("", []string{
		"https://a.example.com/v1/first",
		"https://a.example.com/v1/second",
	}, nil)
	assert.EqualError(t, err, `invalid --provider "https://a.example.com/v1/second": provider name "a.example.com" is already used by another provider`)
}

func TestParse_emptyProviderName(t *testing.T) {
	_, err := Parse("", []string{"https://a.example.com#"}, nil)
	assert.EqualError(t, err, `invalid --provider "https://a.example.com#": empty provider name after '#'`)
}

func TestParse_emptyProviderURL(t *testing.T) {
	_, err := Parse("", []string{"#primary"}, nil)
	assert.EqualError(t, err, `invalid --provider "#primary": empty endpoint URL`)
}

func TestParse_invalidProviderURL(t *testing.T) {
	_, err := Parse("", []string{"://nope#primary"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --provider "://nope#primary": unable to parse endpoint URL`)
}

func TestParse_providerURLWithoutHost(t *testing.T) {
	_, err := Parse("", []string{"localhost:8545"}, nil)
	assert.EqualError(t, err, `invalid --provider "localhost:8545": endpoint URL has no host, it must be an absolute URL like "https://host/path"`)
}

func TestParse_positionalURLIsValidated(t *testing.T) {
	_, err := Parse("localhost:8545", nil, nil)
	assert.EqualError(t, err, `invalid <rpc-endpoint> argument "localhost:8545": endpoint URL has no host, it must be an absolute URL like "https://host/path"`)
}

func TestParse_globalHeadersAppliedToEveryProvider(t *testing.T) {
	providers, err := Parse("", []string{
		"https://a.example.com#a",
		"https://b.example.com#b",
	}, []string{
		"X-Api-Key: secret",
		"User-Agent: fireeth",
	})
	require.NoError(t, err)

	expected := []Header{{Key: "X-Api-Key", Value: "secret"}, {Key: "User-Agent", Value: "fireeth"}}
	assert.Equal(t, []Provider{
		{Name: "a", URL: "https://a.example.com", Headers: expected},
		{Name: "b", URL: "https://b.example.com", Headers: expected},
	}, providers)
}

func TestParse_scopedHeaderAppliedToSingleProvider(t *testing.T) {
	providers, err := Parse("", []string{
		"https://a.example.com#a",
		"https://b.example.com#b",
	}, []string{
		"X-Global: everywhere",
		"Authorization: Bearer token-b#b",
	})
	require.NoError(t, err)

	assert.Equal(t, []Provider{
		{Name: "a", URL: "https://a.example.com", Headers: []Header{
			{Key: "X-Global", Value: "everywhere"},
		}},
		{Name: "b", URL: "https://b.example.com", Headers: []Header{
			{Key: "X-Global", Value: "everywhere"},
			{Key: "Authorization", Value: "Bearer token-b"},
		}},
	}, providers)
}

func TestParse_headerScopedToPositionalDefaultProvider(t *testing.T) {
	providers, err := Parse("https://a.example.com", []string{"https://b.example.com#b"}, []string{
		"X-Api-Key: only-default#default",
	})
	require.NoError(t, err)

	assert.Equal(t, []Header{{Key: "X-Api-Key", Value: "only-default"}}, providers[0].Headers)
	assert.Empty(t, providers[1].Headers)
}

func TestParse_headerValueKeepsColonsAndIsTrimmed(t *testing.T) {
	providers, err := Parse("https://a.example.com", nil, []string{
		"  X-Trace  :   a:b:c  ",
	})
	require.NoError(t, err)

	assert.Equal(t, []Header{{Key: "X-Trace", Value: "a:b:c"}}, providers[0].Headers)
}

func TestParse_headerSplitsOnLastHash(t *testing.T) {
	providers, err := Parse("", []string{"https://a.example.com#a"}, []string{
		"Authorization: Bearer abc#def#a",
	})
	require.NoError(t, err)

	assert.Equal(t, []Header{{Key: "Authorization", Value: "Bearer abc#def"}}, providers[0].Headers)
}

func TestParse_headerForUnknownProvider(t *testing.T) {
	_, err := Parse("", []string{"https://a.example.com#a"}, []string{
		"Authorization: Bearer token#fallback",
	})
	assert.EqualError(t, err, `invalid --headers "Authorization: Bearer token#fallback": no provider named "fallback", known providers are [a]`)
}

func TestParse_headerWithoutColon(t *testing.T) {
	_, err := Parse("https://a.example.com", nil, []string{"NotAHeader"})
	assert.EqualError(t, err, `invalid --headers "NotAHeader": expected format "Key: Value" or "Key: Value#<provider-name>"`)
}

func TestParse_headerWithEmptyKey(t *testing.T) {
	_, err := Parse("https://a.example.com", nil, []string{" : value"})
	assert.EqualError(t, err, `invalid --headers " : value": empty header key`)
}

func TestParse_headerWithEmptyProviderName(t *testing.T) {
	_, err := Parse("https://a.example.com", nil, []string{"X-Key: value#"})
	assert.EqualError(t, err, `invalid --headers "X-Key: value#": empty provider name after '#'`)
}

func TestRedactedURL(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected string
	}{
		{"no path", "https://eth.example.com", "https://eth.example.com"},
		{"root path", "https://eth.example.com/", "https://eth.example.com/"},
		{"api key in path", "https://primary.example.com/v1/secret-key", "https://primary.example.com/redacted"},
		{"api key in query", "https://eth.example.com?apikey=secret", "https://eth.example.com?redacted"},
		{
			"api key in the middle of the path",
			"https://node.avalanche-mainnet.example.com/6eebf479d18c8fa23ffad796d82c9f148aaac6f4/ext/bc/C/rpc/",
			"https://node.avalanche-mainnet.example.com/redacted",
		},
		{"path and query", "https://eth.example.com/v1/key?apikey=secret", "https://eth.example.com/redacted?redacted"},
		{"basic auth", "https://user:password@eth.example.com/v1/key", "https://redacted@eth.example.com/redacted"},
		{"fragment", "https://eth.example.com/v1/key#frag", "https://eth.example.com/redacted"},
		{"unparsable", "://nope", "(unparsable endpoint URL)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Provider{URL: tt.in}.RedactedURL())
		})
	}
}

func names(providers []Provider) []string {
	out := make([]string, len(providers))
	for i, provider := range providers {
		out[i] = provider.Name
	}
	return out
}
