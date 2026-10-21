package httpclient_test

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/httpclient"
)

func TestNewDoesNotInheritDefaultTransportMutations(t *testing.T) {
	def := http.DefaultTransport.(*http.Transport)
	previous := def.TLSClientConfig
	def.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // simula mutação global de outra biblioteca
	defer func() { def.TLSClientConfig = previous }()

	transport := httpclient.New().Transport.(*http.Transport)
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("o transporte do SDK nunca pode herdar InsecureSkipVerify")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS12 || transport.Proxy == nil {
		t.Fatal("TLS 1.2 e proxy do ambiente esperados")
	}
}
