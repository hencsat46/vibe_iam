package uid

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type ObjectType string

const (
	TypeAccessObject ObjectType = "ao"
	TypeEnvironment  ObjectType = "env"
	TypeResource     ObjectType = "res"
	TypeRole         ObjectType = "rol"
)

func New(t ObjectType, systemID, envName string) string {
	random := strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	return fmt.Sprintf("%s:%s:%s:%s", t, sanitize(systemID), sanitize(envName), random)
}

type UID struct {
	Type     ObjectType
	SystemID string
	EnvName  string
	Random   string
}

func (u UID) String() string {
	return fmt.Sprintf("%s:%s:%s:%s", u.Type, u.SystemID, u.EnvName, u.Random)
}

func Parse(s string) (UID, error) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) != 4 {
		return UID{}, fmt.Errorf("uid: invalid format %q: expected type:system:env:random", s)
	}
	for i, p := range parts {
		if p == "" {
			return UID{}, fmt.Errorf("uid: segment %d is empty in %q", i, s)
		}
	}
	return UID{
		Type:     ObjectType(parts[0]),
		SystemID: parts[1],
		EnvName:  parts[2],
		Random:   parts[3],
	}, nil
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9_-]`)

func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "_")
	if s == "" {
		return "x"
	}
	return s
}
