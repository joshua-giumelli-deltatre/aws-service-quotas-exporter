package servicequotas

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/glue"
	"github.com/aws/aws-sdk-go/service/glue/glueiface"
	"github.com/pkg/errors"
)

const (
	jobsPerTriggerName        = "glue_jobs_per_trigger"
	jobsPerTriggerDescription = "glue jobs per trigger"

	jobsName        = "glue_jobs_per_account"
	jobsDescription = "glue jobs per account"

	concurrentRunsPerJobName        = "concurrent_runs_per_glue_job"
	concurrentRunsPerJobDescription = "concurrent runs per glue job"

	dPUsName        = "dpus_per_account"
	dPUsDescription = "DPUs per account"

	concurrentRunsName        = "concurrent_running_glue_jobs"
	concurrentRunsDescription = "concurrent running glue jobs"
)

type JobsPerTriggerCheck struct {
	client glueiface.GlueAPI
}

func (c *JobsPerTriggerCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	// Need to list all the triggers then count the jobs for each trigger

	var triggersList []*string
	listParams := &glue.ListTriggersInput{}
	listErr := c.client.ListTriggersPages(listParams,
		func(page *glue.ListTriggersOutput, lastPage bool) bool {
			if page != nil {
				triggersList = append(triggersList, page.TriggerNames...)
			}
			return !lastPage
		},
	)
	if listErr != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", listErr)
	}

	params := &glue.BatchGetTriggersInput{
		TriggerNames: triggersList,
	}
	triggers, err := c.client.BatchGetTriggers(params)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", listErr)
	}
	for _, trigger := range triggers.Triggers {
		var jobsTriggered int
		for _, action := range trigger.Actions {
			if *action.JobName != "" {
				jobsTriggered++
			}
		}
		usage := QuotaUsage{
			Name:         jobsPerTriggerName,
			Description:  jobsPerTriggerDescription,
			ResourceName: trigger.Name,
			Usage:        float64(jobsTriggered),
		}
		quotaUsages = append(quotaUsages, usage)

	}

	return quotaUsages, nil
}

type JobsPerAccountCheck struct {
	client glueiface.GlueAPI
}

func (c *JobsPerAccountCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var jobsCount int

	params := &glue.ListJobsInput{}
	err := c.client.ListJobsPages(params,
		func(page *glue.ListJobsOutput, lastPage bool) bool {
			if page != nil {
				jobsCount += len(page.JobNames)
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        jobsName,
		Description: jobsDescription,
		Usage:       float64(jobsCount),
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil
}

type ConcurrentRunsPerJobCheck struct {
	client glueiface.GlueAPI
}

func (c *ConcurrentRunsPerJobCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	params := &glue.GetJobsInput{}
	err := c.client.GetJobsPages(params,
		func(page *glue.GetJobsOutput, lastPage bool) bool {
			if page != nil {
				for _, job := range page.Jobs {
					usage := QuotaUsage{
						Name:         concurrentRunsPerJobName,
						Description:  concurrentRunsPerJobDescription,
						ResourceName: job.Name,
						Usage:        float64(*job.ExecutionProperty.MaxConcurrentRuns),
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

type DPUsCheck struct {
	client glueiface.GlueAPI
}

func (c *DPUsCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var dPUsCount int

	params := &glue.GetJobsInput{}
	err := c.client.GetJobsPages(params,
		func(page *glue.GetJobsOutput, lastPage bool) bool {
			if page != nil {
				for _, job := range page.Jobs {
					dPUsCount += int(*job.MaxCapacity)
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        dPUsName,
		Description: dPUsDescription,
		Usage:       float64(dPUsCount),
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil
}

type ConcurrentRunsCheck struct {
	client glueiface.GlueAPI
}

func (c *ConcurrentRunsCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var concurrentJobsCount int

	listParams := &glue.ListJobsInput{}
	listErr := c.client.ListJobsPages(listParams,
		func(page *glue.ListJobsOutput, lastPage bool) bool {
			if page != nil {
				for _, job := range page.JobNames {
					params := &glue.GetJobRunsInput{JobName: job}
					err := c.client.GetJobRunsPages(params,
						func(page *glue.GetJobRunsOutput, lastPage bool) bool {
							if page != nil {
								for _, run := range page.JobRuns {
									if run.JobRunState == aws.String(glue.JobRunStateRunning) {
										concurrentJobsCount++
									}
								}
							}
							return !lastPage
						},
					)
					if err != nil {
						panic(err)
					}
				}
			}
			return !lastPage
		},
	)
	if listErr != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", listErr)
	}
	usage := QuotaUsage{
		Name:        concurrentRunsName,
		Description: concurrentRunsDescription,
		Usage:       float64(concurrentJobsCount),
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}
