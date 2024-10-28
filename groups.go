package carrot

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetGroupsByUser(db *gorm.DB, user *User) ([]Group, error) {
	var members []GroupMember
	result := db.Where("user_id", user.ID).Preload("Group").Find(&members)
	if result.Error != nil {
		return nil, result.Error
	}
	var vals []Group
	for _, v := range members {
		vals = append(vals, v.Group)
	}
	return vals, nil
}

func GetFirstGroupByUser(db *gorm.DB, user *User) (*Group, error) {
	var member GroupMember
	result := db.Where("user_id", user.ID).Preload("Group").Take(&member)
	return &member.Group, result.Error
}

func GetGroupByID(db *gorm.DB, groupID uint) (*Group, error) {
	var val Group
	result := db.Where("id", groupID).Take(&val)
	if result.Error != nil {
		return nil, result.Error
	}
	return &val, nil
}

func CreateGroupByUser(db *gorm.DB, user *User, name string) (*Group, error) {
	group := Group{
		Name: name,
	}
	result := db.Create(&group)
	if result.Error != nil {
		return nil, result.Error
	}

	member := GroupMember{
		UserID:  user.ID,
		GroupID: group.ID,
		Role:    GroupRoleAdmin,
	}
	result = db.Create(&member)
	if result.Error != nil {
		return nil, result.Error
	}
	return &group, nil
}

func SetGroupExtra(db *gorm.DB, group *Group, key string, value string) error {
	extra := GroupExtra{
		ObjectType: "group",
		ObjectID:   group.ID,
		Key:        key,
		Value:      value,
	}
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "object_type"}, {Name: "object_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"key", "value"}),
	}).Create(&extra)
	return result.Error
}

func SetGroupMemberExtra(db *gorm.DB, member *GroupMember, key string, value string) error {
	extra := GroupExtra{
		ObjectType: "member",
		ObjectID:   member.ID,
		Key:        key,
		Value:      value,
	}
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "object_type"}, {Name: "object_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"key", "value"}),
	}).Create(&extra)
	return result.Error
}
