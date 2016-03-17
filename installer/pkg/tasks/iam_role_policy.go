package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/glog"
)

type IAMRolePolicyRenderer interface {
	RenderIAMRolePolicy(actual, expected, changes *IAMRolePolicy) error
}

type IAMRolePolicy struct {
	ID             *string
	Name           *string
	Role           *IAMRole
	PolicyDocument Resource
}

func (s *IAMRolePolicy) Prefix() string {
	return "IAMRolePolicy"
}

func (e *IAMRolePolicy) find(c *Context) (*IAMRolePolicy, error) {
	cloud := c.Cloud

	request := &iam.GetRolePolicyInput{
		RoleName:   e.Role.Name,
		PolicyName: e.Name,
	}

	response, err := cloud.IAM.GetRolePolicy(request)
	if err != nil {
		return nil, fmt.Errorf("error getting role: %v", err)
	}

	p := response
	actual := &IAMRolePolicy{}
	actual.Role = &IAMRole{Name: p.RoleName}
	actual.PolicyDocument = p.PolicyDocument
	actual.Name = p.PolicyName
	return actual, nil
}

func (e *IAMRolePolicy) Run(c *Context) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &IAMRolePolicy{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(IAMRolePolicyRenderer)
	return target.RenderIAMRolePolicy(a, e, changes)
}

func (s *IAMRolePolicy) checkChanges(a, e, changes *IAMRolePolicy) error {
	if a != nil {
		if e.Name == nil {
			return MissingValueError("Name is required when creating IAMRolePolicy")
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderIAMRolePolicy(a, e, changes *IAMRolePolicy) error {
	if a == nil {
		glog.V(2).Infof("Creating IAMRolePolicy")

		request := &iam.PutRolePolicyInput{}
		request.PolicyDocument = e.PolicyDocument
		request.RoleName = e.Name
		request.PolicyName = e.Name

		_, err := t.cloud.IAM.PutRolePolicy(request)
		if err != nil {
			return fmt.Errorf("error creating IAMRolePolicy: %v", err)
		}
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderIAMRolePolicy(a, e, changes *IAMRolePolicy) error {
	t.CreateVar(e)
	if a == nil {
		glog.V(2).Infof("Creating IAMRolePolicy with Name:%q", *e.Name)

		rolePolicyDocument, err := t.AddResource(e.PolicyDocument)
		if err != nil {
			return err
		}

		t.AddIAMCommand("put-role-policy",
			"--role-name", *e.Role.Name,
			"--policy-name", *e.Name,
			"--policy-document", rolePolicyDocument)
	}

	return nil
}
