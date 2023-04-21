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
	CookieJar http.CookieJar
	Scheme    string
	Host      string
}

func NewTestClient(r http.Handler) (c *TestClient) {
	jar, _ := cookiejar.New(nil)
	return &TestClient{
		r:         r,
		CookieJar: jar,
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

	cookies := c.CookieJar.Cookies(currentUrl)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	w := httptest.NewRecorder()
	c.r.ServeHTTP(w, req)
	c.CookieJar.SetCookies(currentUrl, w.Result().Cookies())
	return w
}

// Get return *httptest.ResponseRecoder
func (c *TestClient) Get(path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest("GET", path, nil)
	return c.SendReq(path, req)
}

// Post return *httptest.ResponseRecoder
func (c *TestClient) Post(method, path string, body []byte) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	return c.SendReq(path, req)
}

func (c *TestClient) Call(method, path string, form any, result any) error {
	body, err := json.Marshal(form)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(method, path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := c.SendReq(path, req)
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

func (c *TestClient) CallGet(path string, form, result any) error {
	return c.Call(http.MethodGet, path, form, result)
}

func (c *TestClient) CallPost(path string, form any, result any) error {
	return c.Call(http.MethodPost, path, form, result)
}

func (c *TestClient) CallDelete(path string, form, result any) error {
	return c.Call(http.MethodDelete, path, form, result)
}

func (c *TestClient) CallPut(path string, form, result any) error {
	return c.Call(http.MethodPut, path, form, result)
}

func (c *TestClient) CallPatch(path string, form, result any) error {
	return c.Call(http.MethodPatch, path, form, result)
}
