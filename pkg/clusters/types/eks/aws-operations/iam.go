package awsoperations

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

func createRoles(ctx context.Context, iamClient *iam.Client, namePrefix string) (string, string, error) {
	clusterRoleArn, err := createRole(ctx, iamClient,
		namePrefix+"-EksClusterRole", "Allows access to other AWS service resources that are required to operate clusters managed by EKS.",
		[]string{"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
			"arn:aws:iam::aws:policy/AmazonEKSVPCResourceController"},
		map[string]string{
			"CloudWatchMetricsPolicy": inlinePolicyCloudWatchMetrics,
			"ELBPermissionsPolicy":    inlinePoliciesELBPermissions,
		}, trustedEntitiesEKS,
	)
	if err != nil {
		return "", "", fmt.Errorf("error creating the IAM role for the cluster to use: %w", err)
	}

	nodeRoleArn, err := createRole(ctx, iamClient,
		namePrefix+"-NodeInstanceRole", "Allows EC2 instances to call AWS services on your behalf.",
		[]string{"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
			"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
			"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",
		}, nil, trustedEntitiesEC2,
	)
	if err != nil {
		return "", "", fmt.Errorf("error creating the IAM role for the nodegroup to use: %w", err)
	}
	return clusterRoleArn, nodeRoleArn, nil
}

func createRole(ctx context.Context, iamClient *iam.Client,
	newRoleName string, newRoleDescription string, managedPolicyNames []string, inlinePolicies map[string]string, trustPolicy string) (string, error) {
	input := &iam.CreateRoleInput{
		RoleName:                 aws.String(newRoleName),
		Description:              aws.String(newRoleDescription),
		AssumeRolePolicyDocument: aws.String(trustPolicy),
	}

	roleOutput, err := iamClient.CreateRole(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create role %s: %w", newRoleName, err)
	}

	for name, policy := range inlinePolicies {
		_, err := iamClient.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
			RoleName:       aws.String(newRoleName),
			PolicyDocument: aws.String(policy),
			PolicyName:     aws.String(name),
		})
		if err != nil {
			return "", fmt.Errorf("error adding inline policy %s to role %s: %w", name, newRoleName, err)
		}
	}

	for _, policyName := range managedPolicyNames {
		_, err := iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
			RoleName:  aws.String(newRoleName),
			PolicyArn: aws.String(policyName),
		})
		if err != nil {
			return "", fmt.Errorf("error attaching policy %s to role %s: %w", policyName, newRoleName, err)
		}
	}

	return aws.ToString(roleOutput.Role.Arn), nil
}

func deleteRoles(ctx context.Context, iamClient *iam.Client, roles []string) error {
	const splitter = ":role/"

	for _, roleArn := range roles {
		if roleArn == "" {
			continue
		}
		indexOfPrefix := strings.Index(roleArn, splitter)
		roleName := roleArn[indexOfPrefix+len(splitter):]

		err := detachManagedPolicies(ctx, iamClient, roleName)
		if err != nil {
			return err
		}

		err = deleteInlinePolicies(ctx, iamClient, roleName)
		if err != nil {
			return err
		}

		_, err = iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{
			RoleName: aws.String(roleName),
		})
		if err != nil {
			return fmt.Errorf("failed to delete IAM role %s: %w", roleArn, err)
		}
	}

	return nil
}

func detachManagedPolicies(ctx context.Context, client *iam.Client, roleName string) error {
	listResp, err := client.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return fmt.Errorf("error listing managed policies in role %s: %w", roleName, err)
	}

	for _, policy := range listResp.AttachedPolicies {
		_, err := client.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: policy.PolicyArn,
		})
		if err != nil {
			return fmt.Errorf("error detaching policy %s from role %s: %w", aws.ToString(policy.PolicyArn), roleName, err)
		}
	}

	return nil
}

func deleteInlinePolicies(ctx context.Context, iamClient *iam.Client, roleName string) error {
	listResp, err := iamClient.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return fmt.Errorf("error listing inline policies in role %s: %w", roleName, err)
	}

	for _, policyName := range listResp.PolicyNames {
		_, err := iamClient.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
			RoleName:   aws.String(roleName),
			PolicyName: aws.String(policyName),
		})

		if err != nil {
			return fmt.Errorf("error deleting inline policy %s from role %s: %w", policyName, roleName, err)
		}
	}

	return nil
}

const (
	trustedEntitiesEKS = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "eks.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}`
	trustedEntitiesEC2 = `{
  "Version": "2012-10-17",
  "Statement": [
      {
          "Effect": "Allow",
          "Principal": {
              "Service": "ec2.amazonaws.com"
          },
          "Action": "sts:AssumeRole"
      }
  ]
}`

	inlinePolicyCloudWatchMetrics = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": [
                "cloudwatch:PutMetricData"
            ],
            "Resource": "*",
            "Effect": "Allow"
        }
    ]
}`
	inlinePoliciesELBPermissions = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": [
                "ec2:DescribeAccountAttributes",
                "ec2:DescribeAddresses",
                "ec2:DescribeInternetGateways"
            ],
            "Resource": "*",
            "Effect": "Allow"
        }
    ]
}`
)
