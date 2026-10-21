package credicarro

// ResolvedBaseURL expõe a URL base resolvida para os testes externos.
func ResolvedBaseURL(opts ClientOptions) (string, error) {
	resolved, err := resolveClientOptions(opts)
	if err != nil {
		return "", err
	}
	return resolved.baseURL, nil
}
