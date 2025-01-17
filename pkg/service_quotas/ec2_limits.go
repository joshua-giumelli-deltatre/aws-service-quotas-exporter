package servicequotas

import (
	"math"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/pkg/errors"
)

// Not all quota limits here are reported under "ec2", but all of the
// usage checks are using the ec2 service
const (
	inboundRulesPerSecGrpName = "inbound_rules_per_security_group"
	inboundRulesPerSecGrpDesc = "inbound rules per security group"

	outboundRulesPerSecGrpName = "outbound_rules_per_security_group"
	outboundRulesPerSecGrpDesc = "outbound rules per security group"

	eNIsPerRegionName        = "enis_per_region"
	eNIsPerRegionDescription = "ENIs per region"

	secGroupsPerENIName = "security_groups_per_network_interface"
	secGroupsPerENIDesc = "security groups per network interface"

	securityGroupsPerRegionName = "security_groups_per_region"
	securityGroupsPerRegionDesc = "security groups per region"

	spotInstanceRequestsName = "spot_instance_requests"
	spotInstanceRequestsDesc = "spot instance requests"

	onDemandInstanceRequestsName = "ondemand_instance_requests"
	onDemandInstanceRequestsDesc = "ondemand instance requests"

	availableIPsPerSubnetName = "available_ips_per_subnet"
	availableIPsPerSubnetDesc = "available IPs per subnet"

	maxGp3StoragePerRegionName        = "gp3_storage_per_region"
	maxGp3StoragePerRegionDescription = "GP3 storage per region"

	maxGp2StoragePerRegionName        = "gp2_storage_per_region"
	maxGp2StoragePerRegionDescription = "GP2 storage per region"

	maxIo1StoragePerRegionName        = "io1_storage_per_region"
	maxIo1StoragePerRegionDescription = "IO1 storage per region"

	maxIo2StoragePerRegionName        = "io2_storage_per_region"
	maxIo2StoragePerRegionDescription = "IO2 storage per region"

	maxSt1StoragePerRegionName        = "st1_storage_per_region"
	maxSt1StoragePerRegionDescription = "ST1 storage per region"

	maxStandardStoragePerRegionName        = "standard_storage_per_region"
	maxStandardStoragePerRegionDescription = "standard storage per region"

	maxSc1StoragePerRegionName        = "sc1_storage_per_region"
	maxSc1StoragePerRegionDescription = "SC1 storage per region"

	ebsSnapshotsPerRegionName        = "ebs_snapshots_per_region"
	ebsSnapshotsPerRegionDescription = "EBS snapshots per region"

	maxIo2IopsPerRegionName        = "total_io2_iops_per_region"
	maxIo2IopsPerRegionDescription = "total IO2 IOPS per region"

	maxIo1IopsPerRegionName        = "total_io1_iops_per_region"
	maxIo1IopsPerRegionDescription = "total IO1 IOPS per region"
)

// RulesPerSecurityGroupUsageCheck implements the UsageCheck interface
// for rules per security group
type RulesPerSecurityGroupUsageCheck struct {
	client ec2iface.EC2API
}

// Usage returns the usage for each security group ID with the usage
// value being the sum of their inbound and outbound rules or an error
func (c *RulesPerSecurityGroupUsageCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	params := &ec2.DescribeSecurityGroupsInput{}
	err := c.client.DescribeSecurityGroupsPages(params,
		func(page *ec2.DescribeSecurityGroupsOutput, lastPage bool) bool {
			if page != nil {
				for _, group := range page.SecurityGroups {
					var inboundRules int
					var outboundRules int

					tags := ec2TagsToQuotaUsageTags(group.Tags)

					for _, rule := range group.IpPermissions {
						inboundRules += len(rule.IpRanges)
						inboundRules += len(rule.UserIdGroupPairs)
					}

					inboundUsage := QuotaUsage{
						Name:         inboundRulesPerSecGrpName,
						ResourceName: group.GroupId,
						Description:  inboundRulesPerSecGrpDesc,
						Usage:        float64(inboundRules),
						Tags:         tags,
					}

					for _, rule := range group.IpPermissionsEgress {
						outboundRules += len(rule.IpRanges)
						inboundRules += len(rule.UserIdGroupPairs)
					}

					outboundUsage := QuotaUsage{
						Name:         outboundRulesPerSecGrpName,
						ResourceName: group.GroupId,
						Description:  outboundRulesPerSecGrpDesc,
						Usage:        float64(outboundRules),
						Tags:         tags,
					}

					quotaUsages = append(quotaUsages, []QuotaUsage{inboundUsage, outboundUsage}...)
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

// SecurityGroupsPerENIUsageCheck implements the UsageCheck interface
// for security groups per ENI
type SecurityGroupsPerENIUsageCheck struct {
	client ec2iface.EC2API
}

// Usage returns usage for each Elastic Network Interface ID with the
// usage value being the number of security groups for each ENI or an
// error
func (c *SecurityGroupsPerENIUsageCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	params := &ec2.DescribeNetworkInterfacesInput{}
	err := c.client.DescribeNetworkInterfacesPages(params,
		func(page *ec2.DescribeNetworkInterfacesOutput, lastPage bool) bool {
			if page != nil {
				for _, eni := range page.NetworkInterfaces {
					usage := QuotaUsage{
						Name:         secGroupsPerENIName,
						ResourceName: eni.NetworkInterfaceId,
						Description:  secGroupsPerENIDesc,
						Usage:        float64(len(eni.Groups)),
						Tags:         ec2TagsToQuotaUsageTags(eni.TagSet),
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

// SecurityGroupsPerRegionUsageCheck implements the UsageCheck interface
// for security groups per region
type SecurityGroupsPerRegionUsageCheck struct {
	client ec2iface.EC2API
}

// Usage returns usage for security groups per region as the number of
// all security groups for the region specified with `cfgs` or an error
func (c *SecurityGroupsPerRegionUsageCheck) Usage() ([]QuotaUsage, error) {
	numGroups := 0

	params := &ec2.DescribeSecurityGroupsInput{}
	err := c.client.DescribeSecurityGroupsPages(params,
		func(page *ec2.DescribeSecurityGroupsOutput, lastPage bool) bool {
			if page != nil {
				numGroups += len(page.SecurityGroups)
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}

	usage := []QuotaUsage{
		{
			Name:        securityGroupsPerRegionName,
			Description: securityGroupsPerRegionDesc,
			Usage:       float64(numGroups),
		},
	}
	return usage, nil
}

func standardInstanceTypeFilter() *ec2.Filter {
	return &ec2.Filter{
		Name: aws.String("instance-type"),
		Values: []*string{
			aws.String("a*"),
			aws.String("c*"),
			aws.String("d*"),
			aws.String("h*"),
			aws.String("i*"),
			aws.String("m*"),
			aws.String("r*"),
			aws.String("t*"),
			aws.String("z*"),
		},
	}
}

func activeInstanceFilter() *ec2.Filter {
	return &ec2.Filter{
		Name: aws.String("instance-state-name"),
		Values: []*string{
			aws.String("pending"),
			aws.String("running"),
		},
	}
}

// standardInstancesCPUs returns the number of vCPUs for all standard
// (A, C, D, H, I, M, R, T, Z) EC2 instances
// Note that we are working out the number of vCPUs for each instance
// here because instances can have custom CPU options specified during
// launch. More information can be found at
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-optimize-cpu.html
func standardInstancesCPUs(ec2Service ec2iface.EC2API, spotInstances bool) (int64, error) {
	var totalvCPUs int64
	instanceTypeFilter := standardInstanceTypeFilter()
	instanceStateFilter := activeInstanceFilter()
	filters := []*ec2.Filter{instanceTypeFilter, instanceStateFilter}

	// According to the AWS docs we should be able to filter
	// "scheduled" instances as well, but that does not work so we
	// are using filters only for the spot instances
	if spotInstances {
		spotFilter := &ec2.Filter{
			Name:   aws.String("instance-lifecycle"),
			Values: []*string{aws.String("spot")},
		}
		filters = append(filters, spotFilter)
	}

	params := &ec2.DescribeInstancesInput{Filters: filters}
	err := ec2Service.DescribeInstancesPages(params,
		func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
			if page != nil {
				for _, reservation := range page.Reservations {
					for _, instance := range reservation.Instances {
						// InstanceLifecycle is nil for On-Demand instances
						if !spotInstances && instance.InstanceLifecycle != nil {
							continue
						}

						cpuOptions := instance.CpuOptions
						if cpuOptions.CoreCount != nil && cpuOptions.ThreadsPerCore != nil {
							numvCPUs := *cpuOptions.CoreCount * *cpuOptions.ThreadsPerCore
							totalvCPUs += numvCPUs
						}
					}
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return 0, err
	}

	return totalvCPUs, nil
}

// StandardSpotInstanceRequestsUsageCheck implements the UsageCheck interface
// for standard spot instance requests
type StandardSpotInstanceRequestsUsageCheck struct {
	client ec2iface.EC2API
}

// Usage returns vCPU usage for all standard (A, C, D, H, I, M, R, T,
// Z) spot instance requests and usage or an error
// vCPUs are returned instead of the number of images due to the
// service quota reporting the number of vCPUs
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-spot-limits.html
func (c *StandardSpotInstanceRequestsUsageCheck) Usage() ([]QuotaUsage, error) {
	cpus, err := standardInstancesCPUs(c.client, true)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}

	usage := []QuotaUsage{
		{
			Name:        spotInstanceRequestsName,
			Description: spotInstanceRequestsDesc,
			Usage:       float64(cpus),
		},
	}
	return usage, nil
}

// RunningOnDemandStandardInstancesUsageCheck implements the UsageCheck interface
// for standard on-demand instances
type RunningOnDemandStandardInstancesUsageCheck struct {
	client ec2iface.EC2API
}

// Usage returns vCPU usage for all running on-demand standard (A, C,
// D, H, I, M, R, T, Z) instances or an error vCPUs are returned instead
// of the number of images due to the service quota reporting the number
// of vCPUs
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-spot-limits.html
func (c *RunningOnDemandStandardInstancesUsageCheck) Usage() ([]QuotaUsage, error) {
	cpus, err := standardInstancesCPUs(c.client, false)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}

	usage := []QuotaUsage{
		{
			Name:        onDemandInstanceRequestsName,
			Description: onDemandInstanceRequestsDesc,
			Usage:       float64(cpus),
		},
	}
	return usage, nil
}

// AvailableIpsPerSubnetUsageCheck implements the UsageCheckInterface
// for available IPs per subnet
type AvailableIpsPerSubnetUsageCheck struct {
	client ec2iface.EC2API
}

// Usage returns the usage for each subnet ID with the usage value
// being the number of available IPv4 addresses in that subnet or
// an error
// Note that the Description of the resource here is constructed
// using `availableIPsPerSubnetDesc` defined previously as well as
// the subnet's CIDR block
func (c *AvailableIpsPerSubnetUsageCheck) Usage() ([]QuotaUsage, error) {
	availabilityInfos := []QuotaUsage{}
	var conversionErr error

	params := &ec2.DescribeSubnetsInput{}
	err := c.client.DescribeSubnetsPages(params,
		func(page *ec2.DescribeSubnetsOutput, lastPage bool) bool {
			if page != nil {
				for _, subnet := range page.Subnets {
					cidrBlock := *subnet.CidrBlock
					blockedBits, err := strconv.Atoi(cidrBlock[len(cidrBlock)-2:])
					if err != nil {
						conversionErr = errors.Wrapf(ErrFailedToConvertCidr, "%w", err)
						// stops paging if strconv experiences an error
						return true
					}
					maxNumOfIPs := math.Pow(2, 32-float64(blockedBits))
					usage := float64(maxNumOfIPs - float64(*subnet.AvailableIpAddressCount))
					availabilityInfo := QuotaUsage{
						Name:         availableIPsPerSubnetName,
						ResourceName: subnet.SubnetId,
						Description:  availableIPsPerSubnetDesc,
						Usage:        usage,
						Quota:        float64(maxNumOfIPs),
						Tags:         ec2TagsToQuotaUsageTags(subnet.Tags),
					}
					availabilityInfos = append(availabilityInfos, availabilityInfo)
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}

	if conversionErr != nil {
		return nil, conversionErr
	}

	return availabilityInfos, nil
}

func ec2TagsToQuotaUsageTags(tags []*ec2.Tag) map[string]string {
	length := len(tags)
	if length == 0 {
		return nil
	}

	out := make(map[string]string, length)
	for _, tag := range tags {
		out[ToPrometheusNamingFormat(*tag.Key)] = *tag.Value
	}

	return out
}

type MaxGP2StoragePerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxGP2StoragePerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalStorageCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("gp2")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalStorageCount += int(*vol.Size) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxGp2StoragePerRegionName,
		Description: maxGp2StoragePerRegionDescription,
		Usage:       float64(totalStorageCount / 1024), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type MaxIo1StoragePerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxIo1StoragePerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalStorageCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("io1")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalStorageCount += int(*vol.Size) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxIo1StoragePerRegionName,
		Description: maxIo1StoragePerRegionDescription,
		Usage:       float64(totalStorageCount / 1024), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type MaxIo2StoragePerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxIo2StoragePerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalStorageCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("io2")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalStorageCount += int(*vol.Size) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxIo2StoragePerRegionName,
		Description: maxIo2StoragePerRegionDescription,
		Usage:       float64(totalStorageCount / 1024), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type MaxGP3StoragePerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxGP3StoragePerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalStorageCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("gp3")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalStorageCount += int(*vol.Size) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxGp3StoragePerRegionName,
		Description: maxGp3StoragePerRegionDescription,
		Usage:       float64(totalStorageCount / 1024), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type MaxSt1StoragePerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxSt1StoragePerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalStorageCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("st1")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalStorageCount += int(*vol.Size) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxSt1StoragePerRegionName,
		Description: maxSt1StoragePerRegionDescription,
		Usage:       float64(totalStorageCount / 1024), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type MaxStandardStoragePerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxStandardStoragePerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalStorageCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("standard")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalStorageCount += int(*vol.Size) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxStandardStoragePerRegionName,
		Description: maxStandardStoragePerRegionDescription,
		Usage:       float64(totalStorageCount / 1024), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type MaxSc1StoragePerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxSc1StoragePerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalStorageCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("sc1")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalStorageCount += int(*vol.Size) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxSc1StoragePerRegionName,
		Description: maxSc1StoragePerRegionDescription,
		Usage:       float64(totalStorageCount / 1024), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type EbsSnapshotsPerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *EbsSnapshotsPerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalSnapshotsCount int

	params := &ec2.DescribeSnapshotsInput{}
	err := c.client.DescribeSnapshotsPages(params,
		func(page *ec2.DescribeSnapshotsOutput, lastPage bool) bool {
			if page != nil {
				totalSnapshotsCount += len(page.Snapshots)
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        ebsSnapshotsPerRegionName,
		Description: ebsSnapshotsPerRegionDescription,
		Usage:       float64(totalSnapshotsCount),
	}
	quotaUsages = append(quotaUsages, usage)
	return quotaUsages, nil
}

type MaxIo2IopsPerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxIo2IopsPerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalIopsCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("io2")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalIopsCount += int(*vol.Iops) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxIo2IopsPerRegionName,
		Description: maxIo2IopsPerRegionDescription,
		Usage:       float64(totalIopsCount), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type MaxIo1IopsPerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *MaxIo1IopsPerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalIopsCount int

	params := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("volume-type"),
				Values: []*string{aws.String("io1")},
			},
		},
	}
	err := c.client.DescribeVolumesPages(params,
		func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
			if page != nil {
				for _, vol := range page.Volumes {
					totalIopsCount += int(*vol.Iops) // Size is in GiB
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        maxIo1IopsPerRegionName,
		Description: maxIo1IopsPerRegionDescription,
		Usage:       float64(totalIopsCount), // The limit is in TiB
	}
	quotaUsages = append(quotaUsages, usage)

	return quotaUsages, nil

}

type ENIsPerRegionCheck struct {
	client ec2iface.EC2API
}

func (c *ENIsPerRegionCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	var totalENIsCount int

	params := &ec2.DescribeNetworkInterfacesInput{}
	err := c.client.DescribeNetworkInterfacesPages(params,
		func(page *ec2.DescribeNetworkInterfacesOutput, lastPage bool) bool {
			if page != nil {
				pageENICount := len(page.NetworkInterfaces)
				totalENIsCount += pageENICount
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	}
	usage := QuotaUsage{
		Name:        eNIsPerRegionName,
		Description: eNIsPerRegionDescription,
		Usage:       float64(totalENIsCount),
	}
	quotaUsages = append(quotaUsages, usage)
	return quotaUsages, nil
}
