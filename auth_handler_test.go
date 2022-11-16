package carrot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestDoGet Quick Test CheckResponse
func checkResponse(t *testing.T, w *httptest.ResponseRecorder) (response map[string]interface{}) {
	assert.Equal(t, http.StatusOK, w.Code)
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	return response
}

func TestCarrotInit(t *testing.T) {
	gin.DisableConsoleColor()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	r := gin.Default()
	err = InitCarrot(db, r)
	assert.Nil(t, err)

	r.GET("/mock_test", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{}) })
	client := NewTestClient(r)
	w := client.Get("/mock_test")
	checkResponse(t, w)
	assert.Equal(t, w.Header().Get("Access-Control-Allow-Origin"), CORS_ALLOW_ALL)
}

func TestSession(t *testing.T) {
	r := gin.Default()
	r.Use(WithCookieSession("hello"))
	r.GET("/mock", func(ctx *gin.Context) {
		s := sessions.Default(ctx)
		s.Set(UserField, "test")
		s.Save()
	})
	client := NewTestClient(r)
	w := client.Get("/mock")
	assert.Contains(t, w.Header(), "Set-Cookie")
	assert.Contains(t, w.Header().Get("Set-Cookie"), SessionField+"=")
}

func TestAuthHandler(t *testing.T) {
	gin.DisableConsoleColor()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	r := gin.Default()
	err = InitCarrot(db, r)
	assert.Nil(t, err)
	client := NewTestClient(r)

	{
		form := RegisterUserForm{}
		err = client.Call("/auth/register", form, nil)
		assert.Contains(t, err.Error(), "'Email' failed on the 'required'")
	}
	{
		form := LoginForm{}
		err = client.Call("/auth/login", form, nil)
		assert.Contains(t, err.Error(), "email is required")
	}
	{
		form := RegisterUserForm{
			Email:    "bob@example.org",
			Password: "hello12345",
		}
		var user User
		err = client.Call("/auth/register", form, &user)
		assert.Nil(t, err)
		assert.Equal(t, user.Email, form.Email)

		err = client.Call("/auth/register", form, &user)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "email has exists")
	}
	{
		form := LoginForm{
			Email:    "bob@example.org",
			Password: "hello12345",
		}
		var user User
		err = client.Call("/auth/login", form, &user)
		assert.Nil(t, err)
		assert.Equal(t, user.Email, form.Email)
		assert.Empty(t, user.Password)
		assert.Equal(t, user.LastLoginIP, "")
	}
	{
		w := client.Get("/auth/info")
		vals := checkResponse(t, w)
		assert.Contains(t, vals, "email")
		assert.Equal(t, vals["email"], "bob@example.org")
	}
	{
		w := client.Get("/auth/logout")
		checkResponse(t, w)
	}
	{
		w := client.Get("/auth/info")
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
	{
		form := LoginForm{
			Email:    "bob@hello.org",
			Password: "-",
		}
		var user User
		err = client.Call("/auth/login", form, &user)
		assert.Contains(t, err.Error(), "user not exists")
	}
	{
		form := LoginForm{
			Email:    "bob@example.org",
			Password: "-",
		}
		var user User
		err = client.Call("/auth/login", form, &user)
		assert.Contains(t, err.Error(), "unauthorized")
	}
	{
		form := LoginForm{
			Email:    "bob@example.org",
			Password: "hello12345",
		}
		SetValue(db, KEY_USER_ACTIVATED, "true")
		var user User
		err = client.Call("/auth/login", form, &user)
		assert.Contains(t, err.Error(), "waiting for activation")
	}
	{
		u, _ := GetUserByEmail(db, "bob@example.org")
		err := UpdateUserFields(db, u, map[string]interface{}{
			"Enabled": false,
		})
		assert.Nil(t, err)

		form := LoginForm{
			Email:    "bob@example.org",
			Password: "hello12345",
		}
		var user User
		err = client.Call("/auth/login", form, &user)
		assert.Contains(t, err.Error(), "user not allow login")
	}
}

func TestAuthPassword(t *testing.T) {
	gin.DisableConsoleColor()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	r := gin.Default()
	err = InitCarrot(db, r)
	assert.Nil(t, err)
	client := NewTestClient(r)
	defer func() {
		db.Where("email", "bob@example.org").Delete(&User{})
	}()
	SetValue(db, KEY_USER_ACTIVATED, "no")
	CreateUser(db, "bob@example.org", "123456")

	form := LoginForm{
		Email:    "bob@example.org",
		Password: "123456",
	}
	var user User
	err = client.Call("/auth/login", form, &user)
	assert.Nil(t, err)
	{
		form := ChangePasswordForm{
			Password: "123456",
		}
		var r bool
		err = client.Call("/auth/change_password", form, &r)
		assert.Nil(t, err)
		assert.True(t, r)
	}
	{
		form := ResetPasswordForm{
			Email: "bob_bad@example.org",
		}
		err = client.Call("/auth/reset_password", form, nil)
		assert.Nil(t, err)
	}

	var hash string
	sid := Sig().Connect(SigUserResetpassword, func(sender interface{}, params ...interface{}) {
		assert.Equal(t, len(params), 3)
		hash = params[0].(string)
	})
	defer func() {
		Sig().Disconnect(SigUserResetpassword, sid)
	}()
	{
		form := ResetPasswordForm{
			Email: "bob@example.org",
		}
		var r map[string]interface{}
		err = client.Call("/auth/reset_password", form, &r)
		assert.Nil(t, err)
		assert.NotEmpty(t, hash)
		assert.Contains(t, r, "expired")
	}
	{
		form := ResetPasswordDoneForm{
			Password: "abc",
			Email:    "bob@example.org",
			Token:    hash,
		}
		var r bool
		err = client.Call("/auth/reset_password_done", form, &r)
		assert.Nil(t, err)
		assert.True(t, r)
	}
	{

		form := LoginForm{
			Email:    "bob@example.org",
			Password: "abc",
		}
		var user User
		err = client.Call("/auth/login", form, &user)
		assert.Nil(t, err)
	}
}

func TestAuthToken(t *testing.T) {
	gin.DisableConsoleColor()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	r := gin.Default()
	err = InitCarrot(db, r)
	assert.Nil(t, err)
	client := NewTestClient(r)
	defer func() {
		db.Where("email", "bob@example.org").Delete(&User{})
	}()
	SetValue(db, KEY_USER_ACTIVATED, "no")
	CreateUser(db, "bob@example.org", "123456")

	form := LoginForm{
		Email:    "bob@example.org",
		Password: "123456",
		Remember: true,
	}
	var user User
	err = client.Call("/auth/login", form, &user)
	assert.Nil(t, err)
	assert.NotEmpty(t, user.AuthToken)
	{
		form := LoginForm{
			Email:     "bob@example.org",
			AuthToken: user.AuthToken,
		}
		var user User
		err = client.Call("/auth/login", form, &user)
		assert.Nil(t, err)
		assert.Empty(t, user.AuthToken)
	}
}
func TestAuthActivation(t *testing.T) {
	gin.DisableConsoleColor()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	r := gin.Default()
	err = InitCarrot(db, r)
	assert.Nil(t, err)
	client := NewTestClient(r)
	defer func() {
		db.Where("email", "bob@example.org").Delete(&User{})
	}()
	CreateUser(db, "bob@example.org", "123456")
	SetValue(db, KEY_USER_ACTIVATED, "yes")

	{
		form := LoginForm{
			Email:    "bob@example.org",
			Password: "123456",
		}
		var user User
		err = client.Call("/auth/login", form, &user)
		assert.Contains(t, err.Error(), "waiting for activation")
	}

	{
		bob, _ := GetUserByEmail(db, "bob@example.org")
		assert.False(t, bob.Actived)

		token := EncodeHashToken(bob, time.Now().Add(-10*time.Second).Unix(), true)
		w := client.Get(fmt.Sprintf("/auth/activation?token=%s&next=https://bad.org", token))
		assert.Equal(t, w.Code, http.StatusForbidden)
	}

	var hash string
	sid := Sig().Connect(SigUserVerifyEmail, func(sender interface{}, params ...interface{}) {
		assert.Equal(t, len(params), 3)
		hash = params[0].(string)
	})
	defer func() {
		Sig().Disconnect(SigUserVerifyEmail, sid)
	}()

	{
		form := ResetPasswordForm{
			Email: "bob@example.org",
		}
		var user User
		err = client.Call("/auth/resend", form, &user)
		assert.Nil(t, err)
		assert.NotEmpty(t, hash)

		w := client.Get("/auth/activation?token=" + hash)
		assert.Equal(t, w.Code, http.StatusFound)
		u, _ := GetUserByEmail(db, "bob@example.org")
		assert.True(t, u.Actived)
	}
}
