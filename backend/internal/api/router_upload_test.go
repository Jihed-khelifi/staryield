package api

import (
	"testing"

	"github.com/labstack/echo/v4"
)

// revisions 3.3: both chat attachment routes accept multipart bodies and get
// the upload timeout; avatars and admin images keep the short request timeout.
func TestChatUploadRoutesAreRecognised(t *testing.T) {
	e := echo.New()
	for path, want := range map[string]bool{
		"/api/v1/chat/sessions/:id/attachments":      true,
		"/api/v1/chat/conversations/:id/attachments": true,
		"/api/v1/me/avatar":                          false,
		"/api/v1/admin/psychics/:id/image":           false,
		"/api/v1/chat/conversations/:id/messages":    false,
	} {
		c := e.NewContext(nil, nil)
		c.SetPath(path)
		if got := isChatUpload(c); got != want {
			t.Errorf("%s: isChatUpload=%v want %v", path, got, want)
		}
	}
	for _, path := range []string{"/api/v1/chat/sessions/:id/attachments", "/api/v1/chat/conversations/:id/attachments"} {
		if !uploadRoutes[path] {
			t.Errorf("%s must lift the JSON body limit", path)
		}
	}
}
