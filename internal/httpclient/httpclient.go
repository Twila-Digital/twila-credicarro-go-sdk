// Package httpclient constrói o http.Client usado pelo SDK: transporte próprio (não o
// http.DefaultTransport global), TLS 1.2 no mínimo e redirects desligados.
package httpclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// New devolve um cliente com transporte próprio (montado do zero, com mais conexões
// ociosas por host e TLS 1.2 no mínimo) e redirects desligados: um 3xx é devolvido como
// resposta, sem reenviar o cabeçalho Authorization para outro destino.
func New() *http.Client {
	// Transporte montado do zero: não herda mutações globais do http.DefaultTransport
	// (InsecureSkipVerify, proxy, dialer) feitas por outras bibliotecas do processo.
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: false},
	}

	return &http.Client{Transport: transport, CheckRedirect: noRedirect}
}

// Prepare devolve uma cópia rasa de um cliente fornecido pelo integrador com os
// redirects desligados (o Transport e o Timeout do integrador são preservados).
func Prepare(client *http.Client) *http.Client {
	if client == nil {
		return New()
	}
	copied := *client
	copied.CheckRedirect = noRedirect
	return &copied
}

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
