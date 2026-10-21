package credicarro_test

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func TestDocumentFileFromBytes(t *testing.T) {
	file, err := credicarro.NewDocumentFileFromBytes([]byte("conteudo"), "comprovante.jpeg")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if file.FileName() != "comprovante.jpeg" || file.Size() != 8 {
		t.Fatalf("arquivo incorreto: %s", file)
	}
	if strings.Contains(file.String(), base64.StdEncoding.EncodeToString([]byte("conteudo"))) {
		t.Fatal("String() não deve expor o conteúdo")
	}
}

func TestDocumentFileAcceptsAllowedExtensionsCaseInsensitively(t *testing.T) {
	for _, name := range []string{"a.pdf", "a.PDF", "a.jpg", "a.JPG", "a.jpeg", "a.png", "a.PnG", "relatório.final.pdf"} {
		if _, err := credicarro.NewDocumentFileFromBytes([]byte("x"), name); err != nil {
			t.Errorf("nome %q deveria ser aceito: %v", name, err)
		}
	}
}

func TestDocumentFileRejectsInvalidNames(t *testing.T) {
	cases := map[string]string{
		"vazio":             "",
		"só espaços":        "   ",
		"sem extensão":      "documento",
		"extensão gif":      "a.gif",
		"extensão docx":     "a.docx",
		"extensão dupla":    "a.pdf.exe",
		"caminho unix":      "pasta/a.pdf",
		"caminho windows":   `pasta\a.pdf`,
		"sobe diretório":    "../a.pdf",
		"nome com 256 char": strings.Repeat("a", 252) + ".pdf",
	}
	for name, fileName := range cases {
		fileName := fileName
		t.Run(name, func(t *testing.T) {
			_, err := credicarro.NewDocumentFileFromBytes([]byte("x"), fileName)
			assertConfigurationError(t, err)
		})
	}
}

func TestDocumentFileNameWithExactly255CharactersIsAccepted(t *testing.T) {
	name := strings.Repeat("a", 251) + ".pdf"
	if len(name) != 255 {
		t.Fatalf("nome de teste com %d caracteres", len(name))
	}
	if _, err := credicarro.NewDocumentFileFromBytes([]byte("x"), name); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestDocumentFileSizeLimits(t *testing.T) {
	_, err := credicarro.NewDocumentFileFromBytes(nil, "a.pdf")
	assertConfigurationError(t, err)
	_, err = credicarro.NewDocumentFileFromBytes([]byte{}, "a.pdf")
	assertConfigurationError(t, err)

	if _, err := credicarro.NewDocumentFileFromBytes([]byte{1}, "a.pdf"); err != nil {
		t.Fatalf("1 byte deveria ser aceito: %v", err)
	}
	if _, err := credicarro.NewDocumentFileFromBytes(make([]byte, credicarro.MaxDocumentFileSize), "a.pdf"); err != nil {
		t.Fatalf("10 MiB deveria ser aceito: %v", err)
	}
	_, err = credicarro.NewDocumentFileFromBytes(make([]byte, credicarro.MaxDocumentFileSize+1), "a.pdf")
	assertConfigurationError(t, err)
}

func TestDocumentFileFromReader(t *testing.T) {
	file, err := credicarro.NewDocumentFileFromReader(strings.NewReader("conteudo"), "a.png")
	if err != nil || file.Size() != 8 {
		t.Fatalf("esperava 8 bytes, veio %v (%v)", file, err)
	}

	_, err = credicarro.NewDocumentFileFromReader(bytes.NewReader(make([]byte, credicarro.MaxDocumentFileSize+10)), "a.png")
	assertConfigurationError(t, err)

	_, err = credicarro.NewDocumentFileFromReader(strings.NewReader(""), "a.png")
	assertConfigurationError(t, err)

	_, err = credicarro.NewDocumentFileFromReader(nil, "a.png")
	assertConfigurationError(t, err)

	_, err = credicarro.NewDocumentFileFromReader(strings.NewReader("x"), "a.exe")
	assertConfigurationError(t, err)
}

func TestDocumentFileFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cnh-original.jpg")
	if err := os.WriteFile(path, []byte("imagem"), 0o600); err != nil {
		t.Fatalf("falha ao preparar o arquivo: %v", err)
	}

	file, err := credicarro.NewDocumentFileFromFile(path, "")
	if err != nil || file.FileName() != "cnh-original.jpg" || file.Size() != 6 {
		t.Fatalf("arquivo incorreto: %v (%v)", file, err)
	}

	renamed, err := credicarro.NewDocumentFileFromFile(path, "cnh.jpg")
	if err != nil || renamed.FileName() != "cnh.jpg" {
		t.Fatalf("nome informado deveria prevalecer: %v (%v)", renamed, err)
	}

	_, err = credicarro.NewDocumentFileFromFile(filepath.Join(t.TempDir(), "nao-existe.pdf"), "")
	if err == nil {
		t.Fatal("arquivo inexistente deveria falhar")
	}
	_, err = credicarro.NewDocumentFileFromFile("", "a.pdf")
	assertConfigurationError(t, err)
}
