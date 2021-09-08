package servicequotas

import (
	"github.com/aws/aws-sdk-go/service/sesv2"
	"github.com/aws/aws-sdk-go/service/sesv2/sesv2iface"
	"github.com/pkg/errors"
)

const (
	maxSendIn24HoursName        = "max_send_in_24_hours"
	maxSendIn24HoursDescription = "max send in 24 hours"
)

type MaxSendIn24HoursCheck struct {
	client sesv2iface.SESV2API
}

func (c *MaxSendIn24HoursCheck) Usage() ([]QuotaUsage, error) {
	quotaUsages := []QuotaUsage{}

	params := &sesv2.GetAccountInput{}
	response, err := c.client.GetAccount(params)
	if err != nil {
		return nil, errors.Wrapf(ErrFailedToGetUsage, "%w", err)
	} else {
		usage := QuotaUsage{
			Name:        maxSendIn24HoursName,
			Description: maxSendIn24HoursDescription,
			Usage:       *response.SendQuota.SentLast24Hours,
			Quota:       *response.SendQuota.Max24HourSend,
		}
		quotaUsages = append(quotaUsages, usage)
	}
	return quotaUsages, nil
}
