package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrInvalidStatus      = errors.New("invalid lifecycle status for this operation")
	ErrAccessObjectDraft  = errors.New("access object must be in DRAFT status")
	ErrDeleteRestricted   = errors.New("can only delete access objects in DRAFT or RETIRED status")
	ErrParentNotFound     = errors.New("parent resource not found")
	ErrResourceNotInObject = errors.New("resource does not belong to this access object")
	ErrCircularRole       = errors.New("circular role dependency detected")
)
