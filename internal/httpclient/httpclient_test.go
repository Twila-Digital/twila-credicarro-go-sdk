package httpclient_test

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/httpclient"
)

func TestNewUsesOwnTransportWithTLS12AndNoRedirects(t *testing.T) {
	client := httpclient.New()
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == http.DefaultTransport.(*http.Transport) {
		t.Fatal("deveria usar um transporte próprio, não o http.DefaultTransport")
	}
	if transport.MaxIdleConnsPerHost != 64 {
		t.Errorf("MaxIdleConnsPerHost = %d, esperava 64", transport.MaxIdleConnsPerHost)
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Error("TLS 1.2 deveria ser o mínimo")
	}
	if err := client.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Errorf("CheckRedirect = %v, esperava http.ErrUseLastResponse", err)
	}
}

func TestPrepareKeepsIntegratorClientUntouched(t *testing.T) {
	custom := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return nil }}
	prepared := httpclient.Prepare(custom)
	if prepared == custom || prepared.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("deveria devolver cópia com redirects desligados")
	}
	if custom.CheckRedirect(nil, nil) != nil {
		t.Fatal("o cliente original não pode ser alterado")
	}
	if httpclient.Prepare(nil) == nil {
		t.Fatal("nil deveria gerar o cliente padrão do SDK")
	}
}
