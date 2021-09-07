package servicequotas

import (
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/pkg/errors"
)

const (
	repositoriesPerRegionName        = "repositories_per_region"
	repositoriesPerRegionDescription = "repositories per region"

	imagesPerRepositoryName        = "images_per_repository"
	imagesPerRepositoryDescription = "images per repository"
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

type ImagesPerRepositoryCheck struct {
	client ecriface.ECRAPI
}

func (c *ImagesPerRepositoryCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var listOfRepositories []*string

	listOfRepositoriesParams := &ecr.DescribeRepositoriesInput{}
	listOfRepositoriesErr := c.client.DescribeRepositoriesPages(listOfRepositoriesParams,
		func(page *ecr.DescribeRepositoriesOutput, lastPage bool) bool {
			if page != nil {
				for _, repo := range page.Repositories {
					listOfRepositories = append(listOfRepositories, repo.RepositoryName)
				}
			}
			return !lastPage
		},
	)
	if listOfRepositoriesErr != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", listOfRepositoriesErr)
	}

	for _, repo := range listOfRepositories {
		var imageCount int
		listOfImagesParams := &ecr.ListImagesInput{RepositoryName: repo}
		listOfImagesErr := c.client.ListImagesPages(listOfImagesParams,
			func(page *ecr.ListImagesOutput, lastPage bool) bool {
				if page != nil {
					imageCount += len(page.ImageIds)
				}
				return !lastPage
			},
		)
		if listOfImagesErr != nil {
			return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", listOfImagesErr)
		}

		usage := []QuotaUsage{
			{
				Name:         imagesPerRepositoryName,
				Description:  imagesPerRepositoryDescription,
				ResourceName: repo,
				Usage:        float64(imageCount),
			},
		}
		quotaUsages = append(quotaUsages, usage...)
	}
	return quotaUsages, nil

}
