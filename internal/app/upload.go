package app

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxImageBytes       = 10 << 20
	maxDocumentBytes    = 50 << 20
	maxRequestBytes     = maxDocumentBytes + (1 << 20)
	maxZIPEntries       = 10_000
	maxZIPMetadataBytes = 1 << 20
)

type uploadType struct {
	mime        string
	extension   string
	maxBytes    int64
	disposition string
	image       bool
}

var (
	allowedUploadTypes = map[string]uploadType{
		"image/jpeg": {mime: "image/jpeg", extension: ".jpg", maxBytes: maxImageBytes, image: true},
		"image/png":  {mime: "image/png", extension: ".png", maxBytes: maxImageBytes, image: true},
		"image/gif":  {mime: "image/gif", extension: ".gif", maxBytes: maxImageBytes, image: true},
		"image/webp": {mime: "image/webp", extension: ".webp", maxBytes: maxImageBytes, image: true},

		"application/pdf":      {mime: "application/pdf", extension: ".pdf", maxBytes: maxDocumentBytes, disposition: "inline"},
		"application/epub+zip": {mime: "application/epub+zip", extension: ".epub", maxBytes: maxDocumentBytes, disposition: "attachment"},
		"text/plain":           {mime: "text/plain", extension: ".txt", maxBytes: maxDocumentBytes, disposition: "inline"},
		"text/markdown":        {mime: "text/markdown", extension: ".md", maxBytes: maxDocumentBytes, disposition: "inline"},
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {
			mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", extension: ".docx", maxBytes: maxDocumentBytes, disposition: "attachment",
		},
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {
			mime: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", extension: ".xlsx", maxBytes: maxDocumentBytes, disposition: "attachment",
		},
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": {
			mime: "application/vnd.openxmlformats-officedocument.presentationml.presentation", extension: ".pptx", maxBytes: maxDocumentBytes, disposition: "attachment",
		},
	}
	storedNamePattern  = regexp.MustCompile(`^[0-9a-f]{32}\.(?:jpg|png|gif|webp|pdf|epub|txt|md|docx|xlsx|pptx)$`)
	errUnsupportedFile = errors.New("unsupported file")
)

type fileTooLargeError struct {
	maxBytes int64
}

func (err *fileTooLargeError) Error() string {
	return "file too large"
}

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择要上传的文件")
		return
	}

	data, fileType, err := readSingleUpload(reader)
	if err != nil {
		var maxErr *http.MaxBytesError
		var sizeErr *fileTooLargeError
		switch {
		case errors.As(err, &maxErr):
			writeError(w, http.StatusRequestEntityTooLarge, "文件不能超过 50 MiB")
		case errors.As(err, &sizeErr):
			writeError(w, http.StatusRequestEntityTooLarge, "文件不能超过 "+sizeLabel(sizeErr.maxBytes))
		case errors.Is(err, errUnsupportedFile):
			writeError(w, http.StatusUnsupportedMediaType, "只支持 JPEG、PNG、GIF、WebP、PDF、EPUB、TXT、MD、DOCX、XLSX 和 PPTX 文件")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	if !fileType.matches(data) {
		writeError(w, http.StatusUnsupportedMediaType, "文件内容与声明的类型不一致")
		return
	}

	name, err := a.persistImage(data, fileType.extension)
	if err != nil {
		a.logger.Error("persist uploaded file", "error", err)
		writeError(w, http.StatusInternalServerError, "文件保存失败，请稍后重试")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"url": cleanBaseURL(a.config.PublicBaseURL) + "/images/" + name,
	})
}

func readSingleUpload(reader *multipart.Reader) ([]byte, uploadType, error) {
	var data []byte
	var declared uploadType
	found := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, uploadType{}, err
		}

		if part.FormName() != "image" || part.FileName() == "" || found {
			_ = part.Close()
			return nil, uploadType{}, errors.New("请求只能包含一个 image 文件")
		}
		found = true

		mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			_ = part.Close()
			return nil, uploadType{}, errUnsupportedFile
		}
		var ok bool
		declared, ok = allowedUploadTypes[strings.ToLower(mediaType)]
		if !ok {
			_ = part.Close()
			return nil, uploadType{}, errUnsupportedFile
		}

		data, err = io.ReadAll(io.LimitReader(part, declared.maxBytes+1))
		closeErr := part.Close()
		if err != nil {
			return nil, uploadType{}, err
		}
		if closeErr != nil {
			return nil, uploadType{}, closeErr
		}
		if int64(len(data)) > declared.maxBytes {
			return nil, uploadType{}, &fileTooLargeError{maxBytes: declared.maxBytes}
		}
		if len(data) == 0 {
			return nil, uploadType{}, errors.New("文件不能为空")
		}
	}
	if !found {
		return nil, uploadType{}, errors.New("请选择要上传的文件")
	}
	return data, declared, nil
}

func (kind uploadType) matches(data []byte) bool {
	if kind.image {
		detected, ok := detectImageType(data)
		return ok && detected.mime == kind.mime
	}

	switch kind.mime {
	case "application/pdf":
		return bytes.HasPrefix(data, []byte("%PDF-"))
	case "text/plain", "text/markdown":
		return utf8.Valid(data)
	case "application/epub+zip":
		return validEPUB(data)
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return validOpenXML(data, "word/document.xml")
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return validOpenXML(data, "xl/workbook.xml")
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return validOpenXML(data, "ppt/presentation.xml")
	default:
		return false
	}
}

func detectImageType(data []byte) (uploadType, bool) {
	kind, ok := allowedUploadTypes[http.DetectContentType(data)]
	return kind, ok && kind.image
}

func validEPUB(data []byte) bool {
	reader, ok := openSafeZIP(data)
	if !ok {
		return false
	}
	file := findZIPFile(reader, "mimetype")
	if file == nil || file.Method != zip.Store {
		return false
	}
	mimetype, ok := readSafeZIPFile(reader, "mimetype")
	if !ok || string(mimetype) != "application/epub+zip" {
		return false
	}
	_, ok = readSafeZIPFile(reader, "META-INF/container.xml")
	return ok
}

func validOpenXML(data []byte, documentPart string) bool {
	reader, ok := openSafeZIP(data)
	if !ok {
		return false
	}
	if _, ok := readSafeZIPFile(reader, "[Content_Types].xml"); !ok {
		return false
	}
	_, ok = readSafeZIPFile(reader, documentPart)
	return ok
}

func openSafeZIP(data []byte) (*zip.Reader, bool) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(reader.File) > maxZIPEntries {
		return nil, false
	}
	for _, file := range reader.File {
		cleanName := strings.TrimSuffix(file.Name, "/")
		if cleanName == "" || len(file.Name) > 1024 || strings.ContainsRune(file.Name, '\x00') || path.Clean(cleanName) != cleanName {
			return nil, false
		}
	}
	return reader, true
}

func readSafeZIPFile(reader *zip.Reader, name string) ([]byte, bool) {
	file := findZIPFile(reader, name)
	if file == nil || file.UncompressedSize64 > maxZIPMetadataBytes {
		return nil, false
	}
	contents, err := file.Open()
	if err != nil {
		return nil, false
	}
	defer contents.Close()
	data, err := io.ReadAll(io.LimitReader(contents, maxZIPMetadataBytes+1))
	return data, err == nil && len(data) <= maxZIPMetadataBytes
}

func findZIPFile(reader *zip.Reader, name string) *zip.File {
	for _, file := range reader.File {
		if file.Name == name {
			return file
		}
	}
	return nil
}

func sizeLabel(bytes int64) string {
	return strconv.FormatInt(bytes/(1<<20), 10) + " MiB"
}

func (a *App) persistImage(data []byte, extension string) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return "", err
		}
		name := hex.EncodeToString(random) + extension
		path := filepath.Join(a.config.UploadDir, name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return "", err
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return "", err
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			return "", err
		}
		return name, nil
	}
	return "", errors.New("无法生成唯一文件名")
}

func (a *App) handleImage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !storedNamePattern.MatchString(name) {
		http.NotFound(w, r)
		return
	}

	file, err := os.Open(filepath.Join(a.config.UploadDir, name))
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.logger.Error("open public file", "error", err)
		http.Error(w, "文件读取失败", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	kind, ok := uploadTypeForExtension(filepath.Ext(name))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", kind.mime)
	if !kind.image {
		disposition := kind.disposition
		if disposition == "" {
			disposition = "inline"
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func uploadTypeForExtension(extension string) (uploadType, bool) {
	for _, kind := range allowedUploadTypes {
		if kind.extension == extension {
			return kind, true
		}
	}
	return uploadType{}, false
}
