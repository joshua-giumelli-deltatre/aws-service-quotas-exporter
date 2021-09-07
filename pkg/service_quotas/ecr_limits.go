package servicequotas

import (
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/pkg/errors"
)

const (
	repositoriesPerRegionName        = "repositories_per_region"
	repositoriesPerRegionDescription = "repositories per region"
)

type RepositoriesPerRegionCheck struct {
	client ecriface.ECRAPI
}

func (c *RepositoriesPerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var repositoryCount int

	params := &ecr.DescribeRepositoriesInput{}
	err := c.client.DescribeRepositoriesPages(params,
		func(page *ecr.DescribeRepositoriesOutput, lastPage bool) bool {
			if page != nil {
				repositoryCount += len(page.Repositories)
				usage := QuotaUsage{
					Name:        repositoriesPerRegionName,
					Description: repositoriesPerRegionDescription,
					Usage:       float64(repositoryCount),
				}

				quotaUsages = append(quotaUsages, usage)
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	return quotaUsages, nil
}
