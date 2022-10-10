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
	u.Profile = `{"avatar":"mock_img"}`
	p := u.GetProfile()
	assert.Equal(t, p.Avatar, "mock_img")
}

func TestUserHashToken(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []interface{}{&User{}, &Config{}})
	assert.Nil(t, err)
	bob, _ := CreateUser(db, "bob@example.org", "123456")
	n := time.Now().Add(1 * time.Minute)
	hash := EncodeHashToken(bob, n.Unix())
	u, err := DecodeHashToken(db, hash)
	assert.Nil(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, u.ID, bob.ID)
}
