package carrot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUniqueKey(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []any{&User{}, &Config{}})
	assert.Nil(t, err)
	v := GenUniqueKey(db.Model(User{}), "email", 10)
	assert.Equal(t, len(v), 10)
	v = GenUniqueKey(db.Model(User{}), "xx", 10)
	assert.Equal(t, len(v), 0)
}
