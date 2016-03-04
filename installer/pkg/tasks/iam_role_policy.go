package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/glog"
)

type IAMRolePolicy struct {
	Role           *IAMRole
	Name           string
	PolicyDocument Resource
}

func (r *IAMRolePolicy) String() string {
	return fmt.Sprintf("IAMRolePolicy (name=%s)", r.Name)
}

func (r *IAMRolePolicy) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	request := &iam.GetRolePolicyInput{
		RoleName:   aws.String(r.Role.Name),
		PolicyName: aws.String(r.Name),
	}

	existing, err := cloud.IAM.GetRolePolicy(request)
	if err != nil {
		return fmt.Errorf("error getting role policy: %v", err)
	}

	if existing == nil {
		glog.V(2).Info("Role policy not found; will create: ", r)

		policyDocument, err := output.AddResource(r.PolicyDocument)
		if err != nil {
			return err
		}
		output.AddIAMCommand("put-role-policy",
			"--role-name", r.Role.Name,
			"--policy-name", r.Name,
			"--policy-document", policyDocument)
	}

	return nil
}
