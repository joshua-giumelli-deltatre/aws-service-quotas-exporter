package servicequotas

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/glue"
	"github.com/aws/aws-sdk-go/service/kinesisanalyticsv2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	awsservicequotas "github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/aws/aws-sdk-go/service/servicequotas/servicequotasiface"
	"github.com/aws/aws-sdk-go/service/sesv2"
	"github.com/pkg/errors"
	logging "github.com/sirupsen/logrus"
)

// Errors returned from this package
var (
	ErrInvalidRegion       = errors.New("invalid region")
	ErrFailedToListQuotas  = errors.New("failed to list quotas")
	ErrFailedToGetUsage    = errors.New("failed to get usage")
	ErrFailedToConvertCidr = errors.New("failed to convert CIDR block from string to int")
)

func allServices() []string {
	return []string{"ec2", "vpc", "rds", "ecr", "ecs", "logs", "kinesisanalytics", "redshift", "ebs", "glue"}
}

// UsageCheck is an interface for retrieving service quota usage
type UsageCheck interface {
	// Usage returns slice of QuotaUsage or an error
	Usage() ([]QuotaUsage, error)
}

func newUsageChecks(c client.ConfigProvider, cfgs ...*aws.Config) (map[string]UsageCheck, map[string]UsageCheck, []UsageCheck) {

	// all clients that will be used by the usage checks
	ec2Client := ec2.New(c, cfgs...)
	autoscalingClient := autoscaling.New(c, cfgs...)
	rdsClient := rds.New(c, cfgs...)
	ecrClient := ecr.New(c, cfgs...)
	sesv2Client := sesv2.New(c, cfgs...)
	logsClient := cloudwatchlogs.New(c, cfgs...)
	kdaClient := kinesisanalyticsv2.New(c, cfgs...)
	rsClient := redshift.New(c, cfgs...)
	glueClient := glue.New(c, cfgs...)

	serviceQuotasUsageChecks := map[string]UsageCheck{
		"L-0EA8095F": &RulesPerSecurityGroupUsageCheck{ec2Client},
		"L-2AFB9258": &SecurityGroupsPerENIUsageCheck{ec2Client},
		"L-E79EC296": &SecurityGroupsPerRegionUsageCheck{ec2Client},
		"L-34B43A08": &StandardSpotInstanceRequestsUsageCheck{ec2Client},
		"L-1216C47A": &RunningOnDemandStandardInstancesUsageCheck{ec2Client},
		"L-5BC124EF": &ReadReplicasPerMasterCheck{rdsClient},
		"L-DF5E4CA3": &ENIsPerRegionCheck{ec2Client},
		"L-C7B9AAAB": &LogGroupsPerRegionCheck{logsClient},
		"L-7A658B76": &MaxGP3StoragePerRegionCheck{ec2Client},
		"L-D18FCD1D": &MaxGP2StoragePerRegionCheck{ec2Client},
		"L-FD252861": &MaxIo1StoragePerRegionCheck{ec2Client},
		"L-09BD8365": &MaxIo2StoragePerRegionCheck{ec2Client},
		"L-82ACEF56": &MaxSt1StoragePerRegionCheck{ec2Client},
		"L-9CF3C2EB": &MaxStandardStoragePerRegionCheck{ec2Client},
		"L-17AF77E8": &MaxSc1StoragePerRegionCheck{ec2Client},
		"L-309BACF6": &EbsSnapshotsPerRegionCheck{ec2Client},
		"L-8D977E7E": &MaxIo2IopsPerRegionCheck{ec2Client},
		"L-B3A130E6": &MaxIo1IopsPerRegionCheck{ec2Client},
		"L-EEC98450": &JobsPerTriggerCheck{glueClient},
		"L-611FDDE4": &JobsPerAccountCheck{glueClient},
		"L-F574AED9": &ConcurrentRunsPerJobCheck{glueClient},
		"L-08F3B322": &DPUsCheck{glueClient},
		"L-5E4153CA": &ConcurrentRunsCheck{glueClient},
	}

	serviceDefaultUsageChecks := map[string]UsageCheck{
		"L-CFEB8E8D": &RepositoriesPerRegionCheck{ecrClient},
		"L-03A36CE1": &ImagesPerRepositoryCheck{ecrClient},
		"L-3A88E041": &AppKPUUsageCheck{kdaClient},
		"L-3729A2EF": &AppsPerRegionCheck{kdaClient},
		"L-2E428669": &UserSnapshotsPerRegionCheck{rsClient},
	}

	otherUsageChecks := []UsageCheck{
		&AvailableIpsPerSubnetUsageCheck{ec2Client},
		&ASGUsageCheck{autoscalingClient},
		&MaxSendIn24HoursCheck{sesv2Client},
		// &MaxTotalStorageCheck{rdsClient}, //Need to review this check
	}

	return serviceQuotasUsageChecks, serviceDefaultUsageChecks, otherUsageChecks
}

// QuotaUsage represents service quota usage
type QuotaUsage struct {
	// Name is the name of the quota (eg. spot_instance_requests)
	// or the name given to the piece of exported availibility
	// information (eg. available_IPs_per_subnet)
	Name string
	// ResourceName is the name of the resource in case the quota
	// is for multiple resources. As an example for "rules per
	// security group" the ResourceName will be the ARN of the
	// security group.
	ResourceName *string
	// Description is the name of the service quota (eg. "Inbound
	// or outbound rules per security group")
	Description string
	// Usage is the current service quota usage
	Usage float64
	// Quota is the current quota
	Quota float64

	// Tags are the metadata associated with the resource in form of key, value pairs
	Tags map[string]string
}

// Identifier for the service quota. Either the resource name in case
// the quota is for multiple resources or the name of the quota
func (q QuotaUsage) Identifier() string {
	if q.ResourceName != nil {
		return *q.ResourceName
	}
	return q.Name
}

// ServiceQuotas is an implementation for retrieving service quotas
// and their limits
type ServiceQuotas struct {
	session                   *session.Session
	region                    string
	isAwsChina                bool
	quotasService             servicequotasiface.ServiceQuotasAPI
	serviceQuotasUsageChecks  map[string]UsageCheck
	serviceDefaultUsageChecks map[string]UsageCheck
	otherUsageChecks          []UsageCheck
}

// QuotasInterface is an interface for retrieving AWS service
// quotas and usage
type QuotasInterface interface {
	QuotasAndUsage() ([]QuotaUsage, error)
}

// NewServiceQuotas creates a ServiceQuotas for `region` and `profile`
// or returns an error. Note that the ServiceQuotas will only return
// usage and quotas for the service quotas with implemented usage checks
func NewServiceQuotas(region, profile string) (QuotasInterface, error) {
	validRegion, isChina := isValidRegion(region)
	if !validRegion {
		return nil, errors.Wrapf(ErrInvalidRegion, "failed to create ServiceQuotas")
	}

	opts := session.Options{}
	if profile != "" {
		opts = session.Options{
			Profile:                 profile,
			AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
			SharedConfigState:       session.SharedConfigEnable,
		}
	}

	awsSession, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return nil, err
	}

	quotasService := awsservicequotas.New(awsSession, aws.NewConfig().WithRegion(region))
	serviceQuotasChecks, serviceDefaultUsageChecks, otherChecks := newUsageChecks(awsSession, aws.NewConfig().WithRegion(region))

	if isChina {
		logging.Warn("AWS china currently doesn't support service quotas, disabling...")
	}

	quotas := &ServiceQuotas{
		session:                   awsSession,
		region:                    region,
		quotasService:             quotasService,
		serviceQuotasUsageChecks:  serviceQuotasChecks,
		serviceDefaultUsageChecks: serviceDefaultUsageChecks,
		isAwsChina:                isChina,
		otherUsageChecks:          otherChecks,
	}
	return quotas, nil
}

func isValidRegion(region string) (bool, bool) {
	for _, partition := range endpoints.DefaultPartitions() {
		_, ok := partition.Regions()[region]
		if ok {
			return true, partition.ID() == endpoints.AwsCnPartitionID
		}
	}
	return false, false
}

func (s *ServiceQuotas) defaultsForService(service string) ([]QuotaUsage, error) {
	defaultQuotaUsages := []QuotaUsage{}
	var defaultUsageErr error

	params := &awsservicequotas.ListAWSDefaultServiceQuotasInput{ServiceCode: aws.String(service)}
	err := s.quotasService.ListAWSDefaultServiceQuotasPages(params,
		func(page *awsservicequotas.ListAWSDefaultServiceQuotasOutput, lastPage bool) bool {
			if page != nil {
				for _, quota := range page.Quotas {
					if check, ok := s.serviceDefaultUsageChecks[*quota.QuotaCode]; ok {
						defaultUsages, err := check.Usage()
						if err != nil {
							defaultUsageErr = err
							return true
						}
						for _, defaultUsage := range defaultUsages {
							defaultUsage.Quota = *quota.Value
							defaultQuotaUsages = append(defaultQuotaUsages, defaultUsage)
						}
					}
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToListQuotas, "%w", err)
	}

	if defaultUsageErr != nil {
		return nil, defaultUsageErr
	}
	return defaultQuotaUsages, nil
}

func (s *ServiceQuotas) quotasForService(service string) ([]QuotaUsage, error) {
	serviceQuotaUsages := []QuotaUsage{}
	var usageErr error

	params := &awsservicequotas.ListServiceQuotasInput{ServiceCode: aws.String(service)}
	err := s.quotasService.ListServiceQuotasPages(params,
		func(page *awsservicequotas.ListServiceQuotasOutput, lastPage bool) bool {
			if page != nil {
				for _, quota := range page.Quotas {
					if check, ok := s.serviceQuotasUsageChecks[*quota.QuotaCode]; ok { // this only gets the non default quotas
						quotaUsages, err := check.Usage()
						if err != nil {
							usageErr = err
							// stop paging when an error is encountered
							return true
						}

						for _, quotaUsage := range quotaUsages {
							quotaUsage.Quota = *quota.Value
							serviceQuotaUsages = append(serviceQuotaUsages, quotaUsage)
						}
					}
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToListQuotas, "%w", err)
	}

	if usageErr != nil {
		return nil, usageErr
	}

	return serviceQuotaUsages, nil
}

// QuotasAndUsage returns a slice of `QuotaUsage` or an error
func (s *ServiceQuotas) QuotasAndUsage() ([]QuotaUsage, error) {
	allQuotaUsages := []QuotaUsage{}

	if !s.isAwsChina {
		for _, service := range allServices() {
			serviceQuotas, err := s.quotasForService(service)
			if err != nil {
				return nil, err
			}

			for _, quota := range serviceQuotas {
				allQuotaUsages = append(allQuotaUsages, quota)
			}
		}
		for _, service := range allServices() {
			defaultQuotas, err := s.defaultsForService(service)
			if err != nil {
				return nil, err
			}

			for _, quota := range defaultQuotas {
				allQuotaUsages = append(allQuotaUsages, quota)
			}
		}
	}

	for _, check := range s.otherUsageChecks {
		quotas, err := check.Usage()
		if err != nil {
			return nil, err
		}

		for _, quota := range quotas {
			allQuotaUsages = append(allQuotaUsages, quota)
		}
	}

	return allQuotaUsages, nil
}
