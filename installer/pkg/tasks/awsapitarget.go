package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/contrib/installer/pkg/fi"
)

type AWSAPITarget struct {
	cloud *fi.AWSCloud
}

func (t *AWSAPITarget) AddAWSTags(id string, expected map[string]string) error {
	actual, err := t.cloud.GetTags(id)
	if err != nil {
		return fmt.Errorf("unexpected error fetching tags for resource: %v", err)
	}

	missing := map[string]string{}
	for k, v := range expected {
		actualValue, found := actual[k]
		if found && actualValue == v {
			continue
		}
		missing[k] = v
	}

	if len(missing) != 0 {
		request := &ec2.CreateTagsInput{}
		request.Resources = []*string{&id}
		for k, v := range missing {
			request.Tags = append(request.Tags, &ec2.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}

		_, err := t.cloud.EC2.CreateTags(request)
		if err != nil {
			return fmt.Errorf("error adding tags to resource %q: %v", id, err)
		}
	}

	return nil
}
