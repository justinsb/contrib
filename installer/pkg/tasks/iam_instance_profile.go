package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/glog"
)

type IAMInstanceProfile struct {
	Role *IAMRole
	Name string
}

func (r *IAMInstanceProfile) String() string {
	return fmt.Sprintf("IAMInstanceProfile (name=%s)", r.Name)
}

func (r *IAMInstanceProfile) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	request := &iam.GetInstanceProfileInput{InstanceProfileName: aws.String(r.Name)}

	existing, err := cloud.iam.GetInstanceProfile(request)
	if err != nil {
		return fmt.Errorf("error getting instance profile policy: %v", err)
	}

	if existing == nil {
		glog.V(2).Info("Instance profile not found; will create: ", r)

		output.AddIAMCommand("create-instance-profile",
			"--instance-profile-name", r.Name)
		// TODO: Don't assume all-or-nothing creation
		output.AddIAMCommand("add-role-to-instance-profile",
			"--instance-profile-name", r.Name,
			"--role-name", r.Role.Name)
	}

	return nil
}
