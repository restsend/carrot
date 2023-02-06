package carrot

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	v := GetEnv("NOT_EXIST_ENV")
	assert.Empty(t, v)
	defer func() {
		os.Remove(".env")
	}()

	os.WriteFile(".env", []byte("#hello\nxx\n\nNOT_EXIST_ENV = 100 "), 0666)
	v = GetEnv("NOT_EXIST_ENV")
	assert.Equal(t, v, "100")
}

func TestConfig(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	assert.NotNil(t, db)

	err = InitMigrate(db)
	assert.Nil(t, err)
	{
		CheckValue(db, "mock_test", "unittest")
		v := GetValue(db, "mock_test")
		assert.Equal(t, v, "unittest")

		SetValue(db, "mock_test", "mock_test_new")
		// hint cache
		CheckValue(db, "mock_test", "unittest")
		v = GetValue(db, "mock_test")
		assert.Equal(t, v, "mock_test_new")
	}
	{
		CheckValue(db, "mock_int_value", "100")
		v := GetIntValue(db, "mock_int_value", 2)
		assert.Equal(t, v, 100)

		v = GetIntValue(db, "mock_not_exist", 2)
		assert.Equal(t, v, 2)

		CheckValue(db, "mock_not_exist", "hello")
		v = GetIntValue(db, "mock_not_exist", 3)
		assert.Equal(t, v, 3)
	}
}
