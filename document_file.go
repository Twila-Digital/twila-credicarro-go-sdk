package credicarro

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxDocumentFileSize é o tamanho máximo do arquivo de documento (10 MiB, decodificado).
	MaxDocumentFileSize = 10 * 1024 * 1024
	maxFileNameLength   = 255
)

var allowedDocumentExtensions = map[string]bool{".pdf": true, ".jpg": true, ".jpeg": true, ".png": true}

// DocumentFile é um arquivo de documento já validado e codificado em Base64 puro (sem
// prefixo "data:"), pronto para Proposals.UploadDocument. Crie-o pelos construtores
// NewDocumentFileFromBytes, NewDocumentFileFromReader ou NewDocumentFileFromFile.
//
// Regras: nome sem caminho com até 255 caracteres e extensão .pdf, .jpg, .jpeg ou .png;
// conteúdo de 1 byte a 10 MiB. A extensão precisa corresponder ao conteúdo real do arquivo
// (a API valida e responde 422 Document.InvalidContent se divergir).
type DocumentFile struct {
	fileName string
	base64   string
	size     int
}

// NewDocumentFileFromBytes valida e codifica o conteúdo em Base64.
func NewDocumentFileFromBytes(content []byte, fileName string) (DocumentFile, error) {
	if err := validateDocumentFileName(fileName); err != nil {
		return DocumentFile{}, err
	}
	if len(content) == 0 {
		return DocumentFile{}, &ConfigurationError{Message: "O arquivo do documento está vazio."}
	}
	if len(content) > MaxDocumentFileSize {
		return DocumentFile{}, &ConfigurationError{Message: "O arquivo do documento excede o limite de 10 MiB."}
	}
	return DocumentFile{fileName: fileName, base64: base64Encode(content), size: len(content)}, nil
}

// NewDocumentFileFromReader lê o conteúdo de r (no máximo 10 MiB) e o codifica em Base64.
func NewDocumentFileFromReader(r io.Reader, fileName string) (DocumentFile, error) {
	if err := validateDocumentFileName(fileName); err != nil {
		return DocumentFile{}, err
	}
	if r == nil {
		return DocumentFile{}, &ConfigurationError{Message: "O leitor do arquivo do documento é obrigatório."}
	}
	content, err := io.ReadAll(io.LimitReader(r, MaxDocumentFileSize+1))
	if err != nil {
		return DocumentFile{}, fmt.Errorf("falha ao ler o arquivo do documento: %w", err)
	}
	return NewDocumentFileFromBytes(content, fileName)
}

// NewDocumentFileFromFile lê o arquivo em path. fileName define o nome enviado à API;
// quando vazio, usa o nome do próprio arquivo (sem o caminho).
func NewDocumentFileFromFile(path, fileName string) (DocumentFile, error) {
	if strings.TrimSpace(path) == "" {
		return DocumentFile{}, &ConfigurationError{Message: "O caminho do arquivo do documento é obrigatório."}
	}
	if fileName == "" {
		fileName = filepath.Base(path)
	}
	if err := validateDocumentFileName(fileName); err != nil {
		return DocumentFile{}, err
	}

	f, err := os.Open(path)
	if err != nil {
		return DocumentFile{}, fmt.Errorf("falha ao abrir o arquivo do documento: %w", err)
	}
	defer func() { _ = f.Close() }()
	return NewDocumentFileFromReader(f, fileName)
}

// documentFileJSON é o formato de serialização de DocumentFile (valores reais, para filas e cache).
type documentFileJSON struct {
	FileName      string `json:"fileName"`
	Base64Content string `json:"base64Content"`
}

// MarshalJSON serializa o nome e o conteúdo Base64 reais, para persistir o arquivo (fila, cache).
// Ao contrário de String e GoString, a serialização não é mascarada.
func (d DocumentFile) MarshalJSON() ([]byte, error) {
	return json.Marshal(documentFileJSON{FileName: d.fileName, Base64Content: d.base64})
}

// UnmarshalJSON restaura e revalida o arquivo serializado por MarshalJSON.
func (d *DocumentFile) UnmarshalJSON(data []byte) error {
	var raw documentFileJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	decoded, err := base64.StdEncoding.DecodeString(raw.Base64Content)
	if err != nil {
		return &ConfigurationError{Message: "O conteúdo Base64 do arquivo do documento é inválido."}
	}
	file, err := NewDocumentFileFromBytes(decoded, raw.FileName)
	if err != nil {
		return err
	}
	*d = file
	return nil
}

// FileName é o nome do arquivo enviado à API.
func (d DocumentFile) FileName() string { return d.fileName }

// Size é o tamanho do arquivo, em bytes (decodificado).
func (d DocumentFile) Size() int { return d.size }

// String não inclui o conteúdo do arquivo.
func (d DocumentFile) String() string {
	return fmt.Sprintf("DocumentFile{%s, %d bytes}", d.fileName, d.size)
}

func (d DocumentFile) validate() error {
	if d.base64 == "" || d.size == 0 {
		return &ConfigurationError{Message: "O arquivo do documento é obrigatório; crie-o com NewDocumentFileFromBytes, NewDocumentFileFromReader ou NewDocumentFileFromFile."}
	}
	return validateDocumentFileName(d.fileName)
}

func validateDocumentFileName(fileName string) error {
	if strings.TrimSpace(fileName) == "" {
		return &ConfigurationError{Message: "O nome do arquivo do documento é obrigatório."}
	}
	if len(fileName) > maxFileNameLength {
		return &ConfigurationError{Message: "O nome do arquivo do documento deve ter no máximo 255 caracteres."}
	}
	if strings.ContainsAny(fileName, `/\`) {
		return &ConfigurationError{Message: "O nome do arquivo do documento não pode conter caminho."}
	}
	if !allowedDocumentExtensions[strings.ToLower(filepath.Ext(fileName))] {
		return &ConfigurationError{Message: "A extensão do arquivo do documento deve ser .pdf, .jpg, .jpeg ou .png."}
	}
	return nil
}
