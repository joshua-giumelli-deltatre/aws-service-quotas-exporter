package servicequotas

import (
	"github.com/aws/aws-sdk-go/service/kinesisanalyticsv2"
	"github.com/aws/aws-sdk-go/service/kinesisanalyticsv2/kinesisanalyticsv2iface"
	"github.com/pkg/errors"
)

const (
	flinkKPUsPerAppName        = "kpus_per_flink_app"
	flinkKPUsPerAppDescription = "KPUs per flink app"

	appsPerRegionName        = "apps_per_region"
	appsPerRegionDescription = "apps per region"
)

type AppKPUUsageCheck struct {
	client kinesisanalyticsv2iface.KinesisAnalyticsV2API
}

func (c *AppKPUUsageCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	listParams := &kinesisanalyticsv2.ListApplicationsInput{}
	apps, err := c.client.ListApplications(listParams)
	if err != nil {
		log.Error("Failed to get KPUs Usage")
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	// Go doesn't support while loops, so let's make our own
	// First let's get the first page of apps
	repeat := true
	for repeat != false {
		// Then we iterate over each app from that page
		for _, app := range apps.ApplicationSummaries {
			descParams := &kinesisanalyticsv2.DescribeApplicationInput{ApplicationName: app.ApplicationName}
			response, err := c.client.DescribeApplication(descParams)
			if err != nil {
				log.Error("Failed to describe KDA applications")
				return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
			} else {
				usage := QuotaUsage{
					Name:         flinkKPUsPerAppName,
					Description:  flinkKPUsPerAppDescription,
					ResourceName: response.ApplicationDetail.ApplicationName,
					// we have to add 1 here because what the AWS API reports is off by 1 compared to billing, confirmed with AWS support
					Usage: float64(*response.ApplicationDetail.ApplicationConfigurationDescription.FlinkApplicationConfigurationDescription.ParallelismConfigurationDescription.CurrentParallelism + 1),
				}
				quotaUsages = append(quotaUsages, usage)
			}
		}
		// Once we have finished with that page
		// We need to check if we need to get another one
		if apps.NextToken == nil {
			// If it doesn't have a next token, we know to stop here
			repeat = false
		} else {
			// If it does have a NextToken, we need to get the next page of apps
			listParams = &kinesisanalyticsv2.ListApplicationsInput{NextToken: apps.NextToken}
			apps, err = c.client.ListApplications(listParams)
			if err != nil {
				return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
			}

		}

	}

	return quotaUsages, nil
}

type AppsPerRegionCheck struct {
	client kinesisanalyticsv2iface.KinesisAnalyticsV2API
}

func (c *AppsPerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalAppsCount int
	listParams := &kinesisanalyticsv2.ListApplicationsInput{}
	apps, err := c.client.ListApplications(listParams)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	// Go doesn't support while loops, so let's make our own
	// First let's get the first page of apps
	repeat := true
	for repeat != false {

		totalAppsCount += len(apps.ApplicationSummaries)

		// Once we have finished with that page
		// We need to check if we need to get another one
		if apps.NextToken == nil {
			// If it doesn't have a next token, we know to stop here
			repeat = false
		} else {
			// If it does have a NextToken, we need to get the next page of apps
			listParams = &kinesisanalyticsv2.ListApplicationsInput{NextToken: apps.NextToken}
			apps, err = c.client.ListApplications(listParams)
			if err != nil {
				return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
			}
		}
	}
	quota := QuotaUsage{
		Name:        appsPerRegionName,
		Description: appsPerRegionDescription,
		Usage:       float64(totalAppsCount),
	}
	quotaUsages = append(quotaUsages, quota)

	return quotaUsages, nil
}
