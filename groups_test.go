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
func TestGroupExtra(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	InitMigrate(db)
	assert.Nil(t, err)

	u, err := CreateUser(db, "test@example.com", "123456")
	assert.Nil(t, err)

	g := Group{
		Name: "group1",
		Extra: []GroupExtra{
			{Key: "key1", Value: "value1"},
		},
	}
	err = db.Create(&g).Error
	assert.Nil(t, err)

	m := GroupMember{
		UserID:  u.ID,
		GroupID: g.ID,
		Role:    GroupRoleAdmin,
		Extra: []GroupExtra{
			{Key: "key2", Value: "value2"},
		},
	}
	err = db.Create(&m).Error
	assert.Nil(t, err)

	var extras []GroupExtra
	err = db.Find(&extras).Error

	assert.Nil(t, err)
	assert.Equal(t, 2, len(extras))
	assert.Equal(t, "key1", extras[0].Key)
	assert.Equal(t, "value1", extras[0].Value)
	assert.Equal(t, "group", extras[0].ObjectType)

	assert.Equal(t, "key2", extras[1].Key)
	assert.Equal(t, "value2", extras[1].Value)
	assert.Equal(t, "member", extras[1].ObjectType)

	SetGroupExtra(db, &g, "key1", "newvalue1")
	err = db.Find(&extras).Error
	assert.Nil(t, err)
	assert.Equal(t, 2, len(extras))
	assert.Equal(t, "newvalue1", extras[0].Value)

	SetGroupMemberExtra(db, &m, "key2", "newvalue2")
	err = db.Find(&extras).Error
	assert.Nil(t, err)
	assert.Equal(t, 2, len(extras))
	assert.Equal(t, "newvalue2", extras[1].Value)
}
