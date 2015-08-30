package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/glog"
)

type IAMRole struct {
	Name               string
	RolePolicyDocument Resource
}

func (r *IAMRole) String() string {
	return fmt.Sprintf("IAMRole (name=%s)", r.Name)
}

func (r *IAMRole) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	request := &iam.GetRoleInput{RoleName: aws.String(r.Name)}

	existing, err := cloud.iam.GetRole(request)
	if err != nil {
		return fmt.Errorf("error getting role: %v", err)
	}

	if existing == nil {
		glog.V(2).Info("Role not found; will create: ", r)

		rolePolicyDocument, err := output.AddResource(r.RolePolicyDocument)
		if err != nil {
			return err
		}

		output.AddIAMCommand("create-role",
			"--role-name", r.Name,
			"--assume-role-policy-document", rolePolicyDocument)
	}

	return nil
}
