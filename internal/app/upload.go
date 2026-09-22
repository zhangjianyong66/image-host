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
	maxAudioBytes       = 50 << 20
	maxVideoBytes       = 200 << 20
	maxUploadFiles      = 9
	maxRequestBytes     = maxUploadFiles*maxVideoBytes + (2 << 20)
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
		"image/jpeg":      {mime: "image/jpeg", extension: ".jpg", maxBytes: maxImageBytes, image: true},
		"image/png":       {mime: "image/png", extension: ".png", maxBytes: maxImageBytes, image: true},
		"image/gif":       {mime: "image/gif", extension: ".gif", maxBytes: maxImageBytes, image: true},
		"image/webp":      {mime: "image/webp", extension: ".webp", maxBytes: maxImageBytes, image: true},
		"audio/mpeg":      {mime: "audio/mpeg", extension: ".mp3", maxBytes: maxAudioBytes},
		"audio/wav":       {mime: "audio/wav", extension: ".wav", maxBytes: maxAudioBytes},
		"audio/ogg":       {mime: "audio/ogg", extension: ".ogg", maxBytes: maxAudioBytes},
		"audio/mp4":       {mime: "audio/mp4", extension: ".m4a", maxBytes: maxAudioBytes},
		"video/mp4":       {mime: "video/mp4", extension: ".mp4", maxBytes: maxVideoBytes},
		"video/webm":      {mime: "video/webm", extension: ".webm", maxBytes: maxVideoBytes},
		"video/quicktime": {mime: "video/quicktime", extension: ".mov", maxBytes: maxVideoBytes},

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
	storedNamePattern  = regexp.MustCompile(`^[0-9a-f]{32}\.(?:jpg|png|gif|webp|mp3|wav|ogg|m4a|mp4|webm|mov|pdf|epub|txt|md|docx|xlsx|pptx)$`)
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

	type result struct {
		Filename string `json:"filename"`
		Status   string `json:"status"`
		URL      string `json:"url,omitempty"`
		Error    string `json:"error,omitempty"`
	}
	results := make([]result, 0, maxUploadFiles)
	count := 0
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "上传请求格式不正确")
			return
		}
		count++
		if count > maxUploadFiles {
			_ = part.Close()
			writeError(w, http.StatusBadRequest, "最多同时上传 9 个文件")
			return
		}
		filename := part.FileName()
		if part.FormName() != "image" || filename == "" {
			_ = part.Close()
			writeError(w, http.StatusBadRequest, "文件字段必须是 image")
			return
		}
		data, fileType, err := readUploadPart(part)
		closeErr := part.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "请求体不能超过 "+sizeLabel(maxRequestBytes))
				return
			}
			results = append(results, result{Filename: filename, Status: "failed", Error: uploadErrorMessage(err)})
			continue
		}
		if !fileType.matches(data) {
			results = append(results, result{Filename: filename, Status: "failed", Error: "文件内容与声明的类型不一致"})
			continue
		}
		name, err := a.persistImage(data, fileType.extension)
		if err != nil {
			a.logger.Error("persist uploaded file", "error", err)
			results = append(results, result{Filename: filename, Status: "failed", Error: "文件保存失败，请稍后重试"})
			continue
		}
		results = append(results, result{Filename: filename, Status: "uploaded", URL: cleanBaseURL(a.config.PublicBaseURL) + "/images/" + name})
	}
	if count == 0 {
		writeError(w, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"results": results})
}

func readUploadPart(part *multipart.Part) ([]byte, uploadType, error) {
	mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
	if err != nil {
		return nil, uploadType{}, errUnsupportedFile
	}
	declared, ok := allowedUploadTypes[strings.ToLower(mediaType)]
	if !ok {
		return nil, uploadType{}, errUnsupportedFile
	}
	data, err := io.ReadAll(io.LimitReader(part, declared.maxBytes+1))
	if err != nil {
		return nil, uploadType{}, err
	}
	if int64(len(data)) > declared.maxBytes {
		return nil, uploadType{}, &fileTooLargeError{maxBytes: declared.maxBytes}
	}
	if len(data) == 0 {
		return nil, uploadType{}, errors.New("文件不能为空")
	}
	return data, declared, nil
}

func uploadErrorMessage(err error) string {
	var sizeErr *fileTooLargeError
	if errors.As(err, &sizeErr) {
		return "文件不能超过 " + sizeLabel(sizeErr.maxBytes)
	}
	if errors.Is(err, errUnsupportedFile) {
		return "不支持的文件类型"
	}
	return err.Error()
}

func (kind uploadType) matches(data []byte) bool {
	if kind.image {
		detected, ok := detectImageType(data)
		return ok && detected.mime == kind.mime
	}

	switch kind.mime {
	case "audio/mpeg":
		return bytes.HasPrefix(data, []byte("ID3")) || (len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0)
	case "audio/wav":
		return len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE"))
	case "audio/ogg":
		return bytes.HasPrefix(data, []byte("OggS"))
	case "audio/mp4":
		return validISOBaseMedia(data)
	case "video/mp4", "video/quicktime":
		return validISOBaseMedia(data)
	case "video/webm":
		return bytes.HasPrefix(data, []byte{0x1a, 0x45, 0xdf, 0xa3})
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

func validISOBaseMedia(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	// The first box must be a valid size/type pair and an ftyp box.
	return bytes.Equal(data[4:8], []byte("ftyp"))
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
