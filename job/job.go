package job

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// declaring job context struct
type JobContext struct {
	Target                 string
	Remote                 bool
	Docker                 bool
	SkipLocal              bool
	JobID                  string
	StartTime              time.Time
	TargetDir              string
	RootDir                string
	Tag                    string
	RestartDocker          bool
	RemoteHost             string
	RemoteUser             string
	CompressedSizeBytesInt int64
	CompressedSizeMBString string
}

func GenerateJobID() string {
	// gen new random UUID
	u := uuid.New().String()
	parts := strings.Split(u, "-")
	q1 := parts[0] // initial 8-character sequence from UUID
	q2 := parts[1] // 1st 4-character sequence from UUID

	return q1 + q2
}
