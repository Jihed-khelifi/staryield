package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
)

// readUpload reads one multipart file field, bounded by max bytes. The
// declared content type and file name are ignored: services sniff and
// re-encode the content.
func readUpload(c echo.Context, field string, max int64) ([]byte, map[string]string, error) {
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response(), r.Body, max+64*1024)
	if err := r.ParseMultipartForm(max + 64*1024); err != nil {
		return nil, nil, uploadError(err, max)
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	file, header, err := r.FormFile(field)
	if err != nil {
		return nil, nil, apperror.Validation(map[string]string{field: "Choose an image to upload."})
	}
	defer file.Close()
	if header.Size > max {
		return nil, nil, apperror.PayloadTooLarge(fmt.Sprintf("Choose an image smaller than %d MB.", max>>20))
	}
	data, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return nil, nil, apperror.Validation(map[string]string{field: "The file could not be read."})
	}
	if int64(len(data)) > max {
		return nil, nil, apperror.PayloadTooLarge(fmt.Sprintf("Choose an image smaller than %d MB.", max>>20))
	}
	values := map[string]string{}
	for k, v := range r.MultipartForm.Value {
		if len(v) > 0 {
			values[k] = v[0]
		}
	}
	return data, values, nil
}

func uploadError(err error, max int64) error {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return apperror.PayloadTooLarge(fmt.Sprintf("Choose an image smaller than %d MB.", max>>20))
	}
	return apperror.BadRequest("Upload the image as multipart/form-data.")
}

// optionalBool parses ?name=true|false.
func optionalBool(c echo.Context, name string) (*bool, error) {
	switch c.QueryParam(name) {
	case "":
		return nil, nil
	case "true", "1":
		v := true
		return &v, nil
	case "false", "0":
		v := false
		return &v, nil
	}
	return nil, apperror.Validation(map[string]string{name: "Use true or false."})
}
