package app

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type mediaUploadResult struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

func (a *App) handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	a.uploadMedia(w, r, 1)
}

func (a *App) handleMediaBatchUpload(w http.ResponseWriter, r *http.Request) {
	a.uploadMedia(w, r, maxUploadFiles)
}

// uploadMedia stages validated files in a private directory before publishing.
// Only one file's bounded contents are held in memory at a time.
func (a *App) uploadMedia(w http.ResponseWriter, r *http.Request, limit int) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(limit)*maxVideoBytes+(2<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	stagingDir, err := os.MkdirTemp(a.config.UploadDir, ".media-upload-")
	if err != nil {
		a.logger.Error("create upload staging directory", "error", err)
		writeError(w, http.StatusInternalServerError, "文件保存失败，请稍后重试")
		return
	}
	defer func() {
		if err := os.RemoveAll(stagingDir); err != nil {
			a.logger.Error("remove upload staging directory", "error", err)
		}
	}()
	staged := *a
	staged.config.UploadDir = stagingDir
	var names []string
	results := make([]mediaUploadResult, 0, limit)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeMediaReadError(w, err)
			return
		}
		if len(names) == limit {
			writeError(w, http.StatusBadRequest, "上传文件数量超过接口限制")
			return
		}
		filename := part.FileName()
		if part.FormName() != "file" || filename == "" {
			writeError(w, http.StatusBadRequest, "文件字段必须是 file")
			return
		}
		data, kind, err := readUploadPart(part)
		if err != nil {
			writeMediaReadError(w, err)
			return
		}
		if err := part.Close(); err != nil {
			writeMediaReadError(w, err)
			return
		}
		if !kind.matches(data) {
			writeError(w, http.StatusUnsupportedMediaType, "文件内容与声明的类型不一致")
			return
		}
		name, err := staged.persistImage(data, kind.extension)
		if err != nil {
			a.logger.Error("stage uploaded file", "error", err)
			writeError(w, http.StatusInternalServerError, "文件保存失败，请稍后重试")
			return
		}
		names = append(names, name)
		results = append(results, mediaUploadResult{Filename: filename, URL: cleanBaseURL(a.config.PublicBaseURL) + "/images/" + name})
	}
	if len(names) == 0 {
		writeError(w, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	var published []string
	for _, name := range names {
		// Staging and published files share a filesystem. Link never overwrites
		// an existing name, and the deferred staging cleanup removes its link.
		if err := os.Link(filepath.Join(stagingDir, name), filepath.Join(a.config.UploadDir, name)); err != nil {
			a.logger.Error("publish uploaded file", "error", err)
			for _, path := range published {
				if err := os.Remove(path); err != nil {
					a.logger.Error("roll back uploaded file", "error", err)
				}
			}
			writeError(w, http.StatusInternalServerError, "文件保存失败，请稍后重试")
			return
		}
		published = append(published, filepath.Join(a.config.UploadDir, name))
	}
	if limit == 1 {
		writeJSON(w, http.StatusCreated, results[0])
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"items": results})
}

func writeMediaReadError(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	var sizeErr *fileTooLargeError
	switch {
	case errors.As(err, &maxErr):
		writeError(w, http.StatusRequestEntityTooLarge, "上传请求体过大")
	case errors.As(err, &sizeErr):
		writeError(w, http.StatusRequestEntityTooLarge, uploadErrorMessage(err))
	case errors.Is(err, errUnsupportedFile):
		writeError(w, http.StatusUnsupportedMediaType, "不支持的文件类型")
	default:
		writeError(w, http.StatusBadRequest, "文件为空或上传请求格式不正确")
	}
}
