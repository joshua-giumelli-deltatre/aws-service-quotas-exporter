package servicequotas

import (
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
