package carrot

import "errors"

var ErrEmptyPassword = errors.New("empty password")
var ErrEmptyEmail = errors.New("empty email")
var ErrSameEmail = errors.New("same email")
var ErrEmailExists = errors.New("email exists, please use another email")
var ErrUserNotExists = errors.New("user not exists")
var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden access")
var ErrUserNotAllowLogin = errors.New("user not allow login")
var ErrNotActivated = errors.New("user not activated")
var ErrTokenRequired = errors.New("token required")
var ErrInvalidToken = errors.New("invalid token")
var ErrBadToken = errors.New("bad token")
var ErrTokenExpired = errors.New("token expired")
var ErrEmailRequired = errors.New("email required")

var ErrNotFound = errors.New("not found")
var ErrNotChanged = errors.New("not changed")
var ErrInvalidView = errors.New("with invalid view")

var ErrOnlySuperUser = errors.New("only super user can do this")
var ErrInvalidPrimaryKey = errors.New("invalid primary key")
