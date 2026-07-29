package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxImageBytes   = 10 << 20
	maxRequestBytes = maxImageBytes + (1 << 20)
)

type imageType struct {
	mime      string
	extension string
}

var (
	allowedImageTypes = map[string]imageType{
		"image/jpeg": {mime: "image/jpeg", extension: ".jpg"},
		"image/png":  {mime: "image/png", extension: ".png"},
		"image/gif":  {mime: "image/gif", extension: ".gif"},
		"image/webp": {mime: "image/webp", extension: ".webp"},
	}
	storedNamePattern = regexp.MustCompile(`^[0-9a-f]{32}\.(?:jpg|png|gif|webp)$`)
)

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择要上传的图片")
		return
	}

	data, declaredType, err := readSingleImage(reader)
	if err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr), errors.Is(err, errImageTooLarge):
			writeError(w, http.StatusRequestEntityTooLarge, "图片不能超过 10 MiB")
		case errors.Is(err, errUnsupportedImage):
			writeError(w, http.StatusUnsupportedMediaType, "只支持 JPEG、PNG、GIF 和 WebP 图片")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	imageKind, ok := detectImageType(data)
	if !ok || imageKind.mime != declaredType {
		writeError(w, http.StatusUnsupportedMediaType, "文件内容与声明的图片类型不一致")
		return
	}

	name, err := a.persistImage(data, imageKind.extension)
	if err != nil {
		a.logger.Error("persist uploaded image", "error", err)
		writeError(w, http.StatusInternalServerError, "图片保存失败，请稍后重试")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"url": cleanBaseURL(a.config.PublicBaseURL) + "/images/" + name,
	})
}

var (
	errImageTooLarge    = errors.New("image too large")
	errUnsupportedImage = errors.New("unsupported image")
)

func readSingleImage(reader *multipart.Reader) ([]byte, string, error) {
	var data []byte
	var declaredType string
	found := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		if part.FormName() != "image" || part.FileName() == "" || found {
			_ = part.Close()
			return nil, "", errors.New("请求只能包含一个 image 图片文件")
		}
		found = true

		mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			_ = part.Close()
			return nil, "", errUnsupportedImage
		}
		declared, ok := allowedImageTypes[strings.ToLower(mediaType)]
		if !ok {
			_ = part.Close()
			return nil, "", errUnsupportedImage
		}
		declaredType = declared.mime

		data, err = io.ReadAll(io.LimitReader(part, maxImageBytes+1))
		closeErr := part.Close()
		if err != nil {
			return nil, "", err
		}
		if closeErr != nil {
			return nil, "", closeErr
		}
		if len(data) > maxImageBytes {
			return nil, "", errImageTooLarge
		}
		if len(data) == 0 {
			return nil, "", errors.New("图片文件不能为空")
		}
	}
	if !found {
		return nil, "", errors.New("请选择要上传的图片")
	}
	return data, declaredType, nil
}

func detectImageType(data []byte) (imageType, bool) {
	detected := http.DetectContentType(data)
	kind, ok := allowedImageTypes[detected]
	return kind, ok
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
		a.logger.Error("open public image", "error", err)
		http.Error(w, "图片读取失败", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	extension := filepath.Ext(name)
	contentType := map[string]string{
		".jpg":  "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
	}[extension]
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, name, info.ModTime(), file)
}
