package carrot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroups(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	InitMigrate(db)
	assert.Nil(t, err)

	u, err := CreateUser(db, "test@example.com", "123456")
	assert.Nil(t, err)

	gp1, err := CreateGroupByUser(db, u, "group1")
	assert.Nil(t, err)

	gp2, err := CreateGroupByUser(db, u, "group2")
	assert.Nil(t, err)

	gp, err := GetGroupByID(db, gp2.ID)
	assert.Nil(t, err)
	assert.Equal(t, gp2.Name, gp.Name)

	gp, err = GetFirstGroupByUser(db, u)
	assert.Nil(t, err)
	assert.Equal(t, gp1.Name, gp.Name)

	gps, err := GetGroupsByUser(db, u)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(gps))

	// non-exist group
	negp, err := GetGroupByID(db, 999)
	assert.Nil(t, negp)
	assert.NotNil(t, err)
}
