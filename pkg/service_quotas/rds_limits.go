package servicequotas

import (
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/pkg/errors"
)

const (
	numReadReplicasPerMasterName        = "read_replicas_per_master"
	numReadReplicasPerMasterDescription = "read replicas per master"
)

type ReadReplicasPerMasterCheck struct {
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
