package carrot

import "gorm.io/gorm"

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
