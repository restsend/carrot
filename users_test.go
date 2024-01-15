package carrot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestModels(t *testing.T) {
	u := User{
		FirstName: "bob",
	}
	assert.Equal(t, u.GetVisibleName(), "bob")
	u.LastName = "ni"
	u.FirstName = ""
	assert.Equal(t, u.GetVisibleName(), "ni")
	u.DisplayName = "BOB"
	assert.Equal(t, u.GetVisibleName(), "BOB")
	u.Profile = &Profile{
		Avatar: "mock_img",
	}
	p := u.GetProfile()
	assert.Equal(t, p.Avatar, "mock_img")
}

func TestUserHashToken(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []any{&User{}, &Config{}})
	assert.Nil(t, err)
	bob, _ := CreateUser(db, "bob@example.org", "123456")
	n := time.Now().Add(1 * time.Minute)
	hash := EncodeHashToken(bob, n.Unix(), true)
	u, err := DecodeHashToken(db, hash, true)
	assert.Nil(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, u.ID, bob.ID)
}

func TestUserEmptyPassword(t *testing.T) {

	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []any{&User{}, &Config{}})
	assert.Nil(t, err)
	bob, _ := CreateUser(db, "bob@example.org", "")
	assert.Equal(t, bob.Password, "")
	assert.False(t, CheckPassword(bob, ""))
}

func TestUserProfile(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []any{&User{}})
	assert.Nil(t, err)
	bob, _ := CreateUser(db, "bob@example.org", "123456")
	assert.Nil(t, bob.Profile)

	bob.Profile = &Profile{
		Avatar: "mock_img",
	}
	db.Save(bob)

	u, _ := GetUserByEmail(db, "bob@example.org")
	assert.Equal(t, u.Profile.Avatar, "mock_img")

	err = DeactiveUser(db, u)
	assert.Nil(t, err)

	_, err = GetUserByEmail(db, "bob@example.org")
	assert.NotNil(t, err)

	err = DeactiveUser(db, u)
	assert.Nil(t, err)

}
