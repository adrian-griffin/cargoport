package jobcontext

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// declaring job context struct
type JobContext struct {
	Target        string
	Remote        bool
	Docker        bool
	SkipLocal     bool
	JobID         string
	StartTime     time.Time
	TargetDir     string
	RootDir       string
	Tag           string
	RestartDocker bool
}

func GenerateJobID(context JobContext) string {
	return context.Target + "-" + strings.Split(uuid.New().String(), "-")[0]
}
