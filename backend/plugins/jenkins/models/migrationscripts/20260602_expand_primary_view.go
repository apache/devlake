package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

type expandPrimaryView struct{}

type JenkinsJob20260602 struct {
	PrimaryView string `gorm:"type:text"`
}

func (JenkinsJob20260602) TableName() string {
	return "_tool_jenkins_jobs"
}

func (u *expandPrimaryView) Up(baseRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(baseRes, &JenkinsJob20260602{})
}

func (*expandPrimaryView) Version() uint64 {
	return 20260602000000
}

func (*expandPrimaryView) Name() string {
	return "expand primary_view column to text"
}