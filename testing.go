package carrot

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
)

type TestClient struct {
	r         http.Handler
	cookieJar http.CookieJar
	Scheme    string
	Host      string
}

func NewTestClient(r http.Handler) (c *TestClient) {
	jar, _ := cookiejar.New(nil)
	return &TestClient{
		r:         r,
		cookieJar: jar,
		Scheme:    "http",
		Host:      "1.2.3.4",
	}
}

func (c *TestClient) SendReq(path string, req *http.Request) *httptest.ResponseRecorder {
	req.URL.Scheme = "http"
	req.URL.Host = "MOCKSERVER"
	req.RemoteAddr = "127.0.0.1:1234"

	currentUrl := &url.URL{
		Scheme: c.Scheme,
		Host:   c.Host,
		Path:   path,
	}

	cookies := c.cookieJar.Cookies(currentUrl)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	w := httptest.NewRecorder()
	c.r.ServeHTTP(w, req)
	c.cookieJar.SetCookies(currentUrl, w.Result().Cookies())
	return w
}

// TestDoGet Quick Test Get
func (c *TestClient) Get(path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest("GET", path, nil)
	return c.SendReq(path, req)
}

func (c *TestClient) PostRaw(method, path string, body []byte) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	return c.SendReq(path, req)
}

// Rpc Call
func (c *TestClient) Call(path string, form interface{}, result interface{}) error {
	body, err := json.Marshal(form)
	if err != nil {
		return err
	}
	w := c.PostRaw(http.MethodPost, path, body)
	defer w.Result().Body.Close()
	data := w.Body.Bytes()
	if w.Code != http.StatusOK {
		if data != nil {
			return errors.New("bad status :" + string(data))
		}
		return errors.New("bad status :" + w.Result().Status)
	}
	return json.Unmarshal(w.Body.Bytes(), &result)
}
