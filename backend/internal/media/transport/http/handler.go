package mediahttp

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"blog/internal/media/app"
	"blog/internal/media/infrastructure/local"
	"blog/internal/platform/httpx"
)

func Upload(service *app.Service, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, service.MaxBytes()+(1<<20))
		reader, err := r.MultipartReader()
		if err != nil {
			httpx.Error(w, 400, "请使用文件上传表单")
			return
		}
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "file" || part.FileName() == "" {
			httpx.Error(w, 400, "请选择图片或视频文件")
			return
		}
		defer part.Close()
		asset, err := service.Upload(part, part.FileName())
		if err != nil {
			var maxBody *http.MaxBytesError
			switch {
			case errors.Is(err, local.ErrTooLarge), errors.As(err, &maxBody):
				httpx.Error(w, 413, "文件超过上传大小限制")
			case errors.Is(err, local.ErrUnsupported):
				httpx.Error(w, 415, "支持 JPG、PNG、GIF、WebP 图片和 MP4、WebM 视频，请检查文件内容")
			default:
				logger.Error("upload failed", "error", err)
				httpx.Error(w, 500, "文件上传失败")
			}
			return
		}
		// Clients submit one file per request; multiple selections are uploaded in order.
		if _, err := reader.NextPart(); err != io.EOF {
			if cleanupErr := service.Discard(asset.URL); cleanupErr != nil {
				logger.Error("discard invalid upload", "error", cleanupErr)
			}
			httpx.Error(w, 400, "每个请求只能上传一个文件")
			return
		}
		httpx.JSON(w, 201, asset)
	})
}
