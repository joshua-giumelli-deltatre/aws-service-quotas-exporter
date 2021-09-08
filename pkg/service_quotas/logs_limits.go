package servicequotas

import (
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/pkg/errors"
)

const (
	logGroupsPerRegionName        = "log_groups_per_region"
	logGroupsPerRegionDescription = "log groups per region"
)

type LogGroupsPerRegionCheck struct {
	client cloudwatchlogsiface.CloudWatchLogsAPI
}

func (c *LogGroupsPerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalLogGroupsCount int
	params := &cloudwatchlogs.DescribeLogGroupsInput{}
	err := c.client.DescribeLogGroupsPages(params,
		func(page *cloudwatchlogs.DescribeLogGroupsOutput, lastPage bool) bool {
			if page != nil {
				pageLogGroupsCount := len(page.LogGroups)
				totalLogGroupsCount += pageLogGroupsCount
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        logGroupsPerRegionName,
		Description: logGroupsPerRegionDescription,
		Usage:       float64(totalLogGroupsCount),
	}
	quotaUsages = append(quotaUsages, usage)
	return quotaUsages, nil
}
