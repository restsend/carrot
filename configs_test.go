package carrot

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	v := GetEnv("NOT_EXIST_ENV")
	assert.Empty(t, v)
	defer os.Remove(".env")

	os.WriteFile(".env", []byte("#hello\nxx\n\nNOT_EXIST_ENV = 100\nBAD=\nGOOD=XXX"), 0666)
	v = GetEnv("NOT_EXIST_ENV")
	assert.Equal(t, v, "100")
	type testEnv struct {
		NotExistEnv string `env:"NOT_EXIST_ENV"`
		Bad         string `env:"BAD"`
		Good        string `env:"-"`
	}
	var env testEnv
	env.Bad = "abcd"
	env.Good = "1234"
	LoadEnvs(&env)
	assert.Equal(t, env.NotExistEnv, "100")
	assert.Empty(t, env.Bad)
	assert.Equal(t, env.Good, "1234")
}

func TestConfig(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	assert.NotNil(t, db)

	err = InitMigrate(db)
	assert.Nil(t, err)
	{
		CheckValue(db, "mock_test", "unittest", ConfigFormatText, false, false)
		v := GetValue(db, "mock_test")
		assert.Equal(t, v, "unittest")

		SetValue(db, "mock_test", "mock_test_new", ConfigFormatText, false, false)
		// hint cache
		CheckValue(db, "mock_test", "unittest", ConfigFormatText, false, false)
		v = GetValue(db, "mock_test")
		assert.Equal(t, v, "mock_test_new")
	}
	{
		CheckValue(db, "mock_int_value", "100", ConfigFormatText, false, false)
		v := GetIntValue(db, "mock_int_value", 2)
		assert.Equal(t, v, 100)

		v = GetIntValue(db, "mock_not_exist", 2)
		assert.Equal(t, v, 2)

		CheckValue(db, "mock_not_exist", "hello", ConfigFormatText, false, false)
		v = GetIntValue(db, "mock_not_exist", 3)
		assert.Equal(t, v, 3)
	}
	{
		SetValue(db, "mock_test_ex", "unittest", ConfigFormatText, true, true)
		LoadAutoloads(db)
		v := GetValue(db, "mock_test_ex")
		assert.Equal(t, v, "unittest")

		vals := LoadPublicConfigs(db)
		assert.Equal(t, len(vals), 1)
		assert.Equal(t, vals[0].Value, "unittest")
	}
}
