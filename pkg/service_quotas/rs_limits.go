package servicequotas

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/pkg/errors"
)

const (
	userSnapshotsPerRegionName        = "user_snapshots_per_region"
	userSnapshotsPerRegionDescription = "user snapshots per region"
)

type UserSnapshotsPerRegionCheck struct {
	client redshiftiface.RedshiftAPI
}

func (c *UserSnapshotsPerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var userSnapshotsCount int

	params := &redshift.DescribeClusterSnapshotsInput{SnapshotType: aws.String("manual")}
	err := c.client.DescribeClusterSnapshotsPages(params,
		func(page *redshift.DescribeClusterSnapshotsOutput, lastPage bool) bool {
			if page != nil {
				userSnapshotsCount += len(page.Snapshots)
			}
			return !lastPage
		},
	)
	if err != nil {
		log.Error("Failed to get Redshift Snapshots Usage Check")
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        userSnapshotsPerRegionName,
		Description: userSnapshotsPerRegionDescription,
		Usage:       float64(userSnapshotsCount),
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}
