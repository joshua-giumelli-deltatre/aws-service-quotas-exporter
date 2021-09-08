package servicequotas

import (
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/pkg/errors"
)

const (
	numReadReplicasPerMasterName        = "read_replicas_per_master"
	numReadReplicasPerMasterDescription = "read replicas per master"

	MaxTotalStorageCheckName        = "max_total_storage"
	MaxTotalStorageCheckDescription = "max total storage"
)

type ReadReplicasPerMasterCheck struct {
	client rdsiface.RDSAPI
}

type MaxTotalStorageCheck struct {
	client rdsiface.RDSAPI
}

func (c *ReadReplicasPerMasterCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	params := &rds.DescribeDBClustersInput{}
	err := c.client.DescribeDBClustersPages(params,
		func(page *rds.DescribeDBClustersOutput, lastPage bool) bool {
			if page != nil {
				for _, group := range page.DBClusters {
					var readReplicas int

					for _, clusterMember := range group.DBClusterMembers {
						if !*clusterMember.IsClusterWriter {
							readReplicas++
						}
					}

					usage := QuotaUsage{
						Name:         numReadReplicasPerMasterName,
						ResourceName: group.DBClusterIdentifier,
						Description:  numReadReplicasPerMasterDescription,
						Usage:        float64(readReplicas),
						// Quota:        float64(5), Set the actual value here
					}

					quotaUsages = append(quotaUsages, usage)
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}

	return quotaUsages, nil
}

func (c *MaxTotalStorageCheck) Usage() ([]QuotaUsage, error) {
	quotasUsage := []QuotaUsage{}

	var totalStorageCount int64

	params := &rds.DescribeDBInstancesInput{}
	err := c.client.DescribeDBInstancesPages(params,
		func(page *rds.DescribeDBInstancesOutput, lastPage bool) bool {
			if page != nil {
				for _, instance := range page.DBInstances {
					totalStorageCount += int64(*instance.AllocatedStorage)
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}

	usage := QuotaUsage{
		Name:        MaxTotalStorageCheckName,
		Description: MaxTotalStorageCheckDescription,
		Usage:       float64(totalStorageCount),
	}

	quotasUsage = append(quotasUsage, usage)

	return quotasUsage, nil
}
