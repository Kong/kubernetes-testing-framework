package aws_operations

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/weaveworks/eksctl/pkg/ami"
	eksctlapi "github.com/weaveworks/eksctl/pkg/apis/eksctl.io/v1alpha5"
	"github.com/weaveworks/eksctl/pkg/authconfigmap"
	eksiam "github.com/weaveworks/eksctl/pkg/iam"
	"github.com/weaveworks/eksctl/pkg/nodebootstrap"
	"k8s.io/client-go/kubernetes"
	"os"
	"time"
)

const (
	DefaultNodeGroupName     = "default-node-group"
	DefaultKubernetesSvcCIDR = "172.20.0.0/16"
	kubernetesTagFormat      = "kubernetes.io/cluster/%s"
	envKeyNodeSSHKeyName     = "EKS_NODE_SSH_KEY"

	TagNameCreateBy = "ktf_created_by"
)

// CreateEKSClusterAll create an EKS cluster with all the necessary resources
// It creates the cluster by sending direct API calls to create AWS resources instead of setting up CloudFormation stacks
// Make sure you've set the correct AWS credentials and the caller identity has correct permissions to create these resources
// More information: https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html#specifying-credentials
func CreateEKSClusterAll(ctx context.Context, cfg aws.Config,
	clusterName, k8sMinorVersion, nodeMachineType string,
	tags map[string]string) error {

	stsClient := sts.NewFromConfig(cfg)
	ec2Client := ec2.NewFromConfig(cfg)
	eksClient := eks.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)

	callIdentityOutput, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return errors.Wrap(err, "failed to get caller identity")
	}
	createdByArn := aws.ToString(callIdentityOutput.Arn)

	clusterRoleArn, nodeRoleArn, err := createRoles(ctx, iamClient, clusterName)
	if err != nil {
		return errors.Wrap(err, "failed to create IAM roles")
	}
	subnetAvZones, err := getAvailabilityZones(ctx, ec2Client, cfg.Region)
	if err != nil {
		return errors.Wrapf(err, "failed to get availability zones in region %s", cfg.Region)
	}

	vpcId, subnetIDs, err := createVPC(ctx, ec2Client, subnetAvZones)
	if err != nil {
		return errors.Wrap(err, "failed to create VPC")
	}

	cpSgId, err := createControlPlaneSecurityGroup(ctx, ec2Client, vpcId, clusterName)
	if err != nil {
		return errors.Wrapf(err, "failed to create control plane security group in VPC %s", vpcId)
	}

	_, err = createCluster(ctx, eksClient, clusterName, clusterRoleArn, k8sMinorVersion, cpSgId, subnetIDs, createdByArn, tags)
	if err != nil {
		return errors.Wrapf(err, "failed to create EKS cluster %s", clusterName)
	}

	activeCluster, err := waitForClusterActive(ctx, eksClient, clusterName)
	if err != nil {
		return errors.Wrapf(err, "failed while waiting for EKS cluster %s to become active", clusterName)
	}

	sgId, err := createNodeSecurityGroup(ctx, ec2Client, vpcId, clusterName, activeCluster.ResourcesVpcConfig.SecurityGroupIds)
	if err != nil {
		return errors.Wrapf(err, "failed to create security groups")
	}

	_, kubeCfg, err := ClientForCluster(ctx, cfg, clusterName)
	if err != nil {
		return errors.Wrapf(err, "failed to get kube client for cluster %s", clusterName)
	}

	err = authorizeNodeGroup(kubeCfg, nodeRoleArn)
	if err != nil {
		return errors.Wrapf(err, "failed to authorize node group to access cluster %s", clusterName)
	}

	amiId, err := resolveAMI(ctx, ec2Client, cfg.Region, k8sMinorVersion, nodeMachineType, eksctlapi.DefaultNodeImageFamily)
	if err != nil {
		return errors.Wrap(err, "failed to resolve AMI")
	}

	clusterCfg := buildClusterConfig(clusterName, k8sMinorVersion, nodeMachineType, cfg.Region, amiId, subnetAvZones)
	ng := clusterCfg.NodeGroups[0]
	clusterCfg.VPC.ID = vpcId
	ng.Subnets = subnetIDs
	ng.SecurityGroups.AttachIDs = []string{sgId}
	ng.IAM.InstanceRoleARN = nodeRoleArn

	err = clusterCfg.SetClusterState(activeCluster)
	if err != nil {
		return errors.Wrapf(err, "failed to create cluster state object for cluster %s", clusterName)
	}

	err = createNodeGroup(ctx, eksClient, ec2Client, clusterCfg)
	if err != nil {
		return errors.Wrapf(err, "failed to create EKS node group for cluster %s", clusterName)
	}

	return nil
}

// DeleteEKSClusterAll cleans up all created resources of a given existing EKS cluster
// It cleans up the resources introduced during the cluster creation.
// You probably need to clean up the resources manually in the following scenarios:
//  1. the cluster creation was not complete: this function could not collect necessary information from an incomplete state
//  2. the cleanup process fails for some reason
//
// To clean up the resources manually, using the cluster name to search resources in the following AWS services:
// - EKS - Compute - NodeGroup
// - EKS - Cluster
// - EC2 - Launch Template
// - IAM - Roles
// - VPC - Load Balancers
// - VPC - Internet Gateways
// - VPC - Security Groups
// - VPC
func DeleteEKSClusterAll(ctx context.Context, cfg aws.Config, clusterName string) error {
	eksClient := eks.NewFromConfig(cfg)
	ec2Client := ec2.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)

	activeCluster, err := eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return errors.Wrap(err, "failed to read cluster information")
	}

	vpcID := activeCluster.Cluster.ResourcesVpcConfig.VpcId
	ngRole, launchTemplateId, err := deleteNodeGroup(ctx, eksClient, clusterName)
	if err != nil {
		return err
	}
	if launchTemplateId != "" {
		err = deleteNodeLaunchTemplate(ctx, ec2Client, launchTemplateId)
		if err != nil {
			return err
		}
	}

	err = deleteRoles(ctx, iamClient, []string{ngRole, *activeCluster.Cluster.RoleArn})
	if err != nil {
		return err
	}

	err = deleteCluster(ctx, eksClient, clusterName)
	if err != nil {
		return err
	}

	return deleteVPC(ctx, ec2Client, *vpcID)
}

func createCluster(ctx context.Context, eksClient *eks.Client,
	clusterName, clusterRoleArn, version, cpSgId string, subnetIDs []string,
	createdByArn string, tags map[string]string) (*types.Cluster, error) {
	eksCreateInput := &eks.CreateClusterInput{
		Name:    &clusterName,
		RoleArn: &clusterRoleArn,
		Version: aws.String(version),

		AccessConfig: &types.CreateAccessConfigRequest{
			AuthenticationMode:                      types.AuthenticationModeConfigMap,
			BootstrapClusterCreatorAdminPermissions: aws.Bool(true),
		},
		ResourcesVpcConfig: &types.VpcConfigRequest{
			EndpointPrivateAccess: aws.Bool(true),
			EndpointPublicAccess:  aws.Bool(true),
			SubnetIds:             subnetIDs,
			SecurityGroupIds:      []string{cpSgId},
		},
		KubernetesNetworkConfig: &types.KubernetesNetworkConfigRequest{
			ServiceIpv4Cidr: aws.String(DefaultKubernetesSvcCIDR),
		},

		Tags: lo.Assign(
			map[string]string{TagNameCreateBy: createdByArn},
			tags,
		),
	}

	clusterOutput, err := eksClient.CreateCluster(ctx, eksCreateInput)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create EKS cluster %s", clusterName)
	}
	return clusterOutput.Cluster, nil
}

func buildClusterConfig(clusterName, minorVersion, nodeMachineType, region, amiId string, subnetAvZones []string) *eksctlapi.ClusterConfig {
	clusterCfg := eksctlapi.NewClusterConfig()

	clusterCfg.Metadata.Name = clusterName
	clusterCfg.Metadata.Region = region
	clusterCfg.Metadata.Version = minorVersion
	clusterCfg.KubernetesNetworkConfig.ServiceIPv4CIDR = DefaultKubernetesSvcCIDR
	clusterCfg.Status = &eksctlapi.ClusterStatus{}

	ng := clusterCfg.NewNodeGroup()
	ng.Name = DefaultNodeGroupName
	ng.ContainerRuntime = aws.String(eksctlapi.ContainerRuntimeContainerD)
	ng.AMIFamily = eksctlapi.DefaultNodeImageFamily
	ng.AMI = amiId
	ng.InstanceType = nodeMachineType
	ng.AvailabilityZones = subnetAvZones
	ng.ScalingConfig = &eksctlapi.ScalingConfig{
		DesiredCapacity: aws.Int(1),
		MinSize:         aws.Int(1),
		MaxSize:         aws.Int(1),
	}

	nodeKeyName := os.Getenv(envKeyNodeSSHKeyName)
	if nodeKeyName != "" {
		ng.SSH.Allow = aws.Bool(true)
		ng.SSH.PublicKeyName = aws.String(nodeKeyName)
	}

	return clusterCfg
}

func waitForClusterActive(ctx context.Context, eksClient *eks.Client, clusterName string) (*types.Cluster, error) {
	childCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-childCtx.Done():
			return nil, childCtx.Err()
		case <-ticker.C:
			describeInput := &eks.DescribeClusterInput{
				Name: &clusterName,
			}
			resp, err := eksClient.DescribeCluster(ctx, describeInput)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("failed to describe EKS cluster %s", clusterName))
			}

			status := resp.Cluster.Status
			if status == types.ClusterStatusActive {
				return resp.Cluster, nil
			}
		}
	}
}

func authorizeNodeGroup(clientSet kubernetes.Interface, nodeRoleArn string) error {
	acm, err := authconfigmap.NewFromClientSet(clientSet)
	if err != nil {
		return err
	}

	nodeGroupRoles := authconfigmap.RoleNodeGroupGroups

	identity, err := eksiam.NewIdentity(nodeRoleArn, authconfigmap.RoleNodeGroupUsername, nodeGroupRoles)
	if err != nil {
		return err
	}

	if err := acm.AddIdentity(identity); err != nil {
		return errors.Wrap(err, "adding nodegroup to auth ConfigMap")
	}
	if err := acm.Save(); err != nil {
		return errors.Wrap(err, "saving auth ConfigMap")
	}
	return nil
}

func createNodeGroup(ctx context.Context, eksClient *eks.Client, ec2Client *ec2.Client, clusterCfg *eksctlapi.ClusterConfig) error {
	nodeGroup := clusterCfg.NodeGroups[0]
	launchTemplateId, err := createNodeLaunchTemplate(ctx, ec2Client, clusterCfg)
	if err != nil {
		return errors.Wrap(err, "failed to create launch template")
	}

	input := &eks.CreateNodegroupInput{
		ClusterName:   aws.String(clusterCfg.Metadata.Name),
		NodegroupName: aws.String(nodeGroup.Name),
		NodeRole:      aws.String(nodeGroup.IAM.InstanceRoleARN),
		Subnets:       nodeGroup.Subnets,
		ScalingConfig: &types.NodegroupScalingConfig{
			MinSize:     aws.Int32(int32(aws.ToInt(nodeGroup.MinSize))),
			MaxSize:     aws.Int32(int32(aws.ToInt(nodeGroup.MaxSize))),
			DesiredSize: aws.Int32(int32(aws.ToInt(nodeGroup.DesiredCapacity))),
		},
		LaunchTemplate: &types.LaunchTemplateSpecification{
			Id: aws.String(launchTemplateId),
		},
	}

	_, err = eksClient.CreateNodegroup(ctx, input)
	if err != nil {
		return err
	}

	return waitForNodeGroupReady(ctx, eksClient, clusterCfg.Metadata.Name, nodeGroup.Name)
}

func waitForNodeGroupReady(ctx context.Context, eksClient *eks.Client, clusterName, nodeGroupName string) error {
	childCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-childCtx.Done():
			return childCtx.Err()
		case <-ticker.C:
			describeInput := &eks.DescribeNodegroupInput{
				ClusterName:   &clusterName,
				NodegroupName: &nodeGroupName,
			}
			resp, err := eksClient.DescribeNodegroup(ctx, describeInput)
			if err != nil {
				return errors.Wrapf(err, "failed to describe node group %s", nodeGroupName)
			}

			status := resp.Nodegroup.Status
			if status == types.NodegroupStatusActive {
				return nil
			}
		}
	}
}

func createNodeLaunchTemplate(ctx context.Context, ec2Client *ec2.Client, clusterCfg *eksctlapi.ClusterConfig) (string, error) {
	nodeGroup := clusterCfg.NodeGroups[0]
	bootstrap := nodebootstrap.NewAL2Bootstrapper(clusterCfg, nodeGroup, nodeGroup.ClusterDNS)
	userdata, err := bootstrap.UserData()
	if err != nil {
		return "", errors.Wrap(err, "failed to generate instance bootstrap user data")
	}

	input := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(fmt.Sprintf("%s-node-template", clusterCfg.Metadata.Name)),
		LaunchTemplateData: &ec2Types.RequestLaunchTemplateData{
			ImageId:          aws.String(nodeGroup.AMI),
			InstanceType:     ec2Types.InstanceType(nodeGroup.InstanceType),
			SecurityGroupIds: nodeGroup.SecurityGroups.AttachIDs,
			BlockDeviceMappings: []ec2Types.LaunchTemplateBlockDeviceMappingRequest{
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2Types.LaunchTemplateEbsBlockDeviceRequest{
						VolumeSize: aws.Int32(int32(aws.ToInt(nodeGroup.VolumeSize))),
						VolumeType: ec2Types.VolumeType(aws.ToString(nodeGroup.VolumeType)),
					},
				},
			},
			UserData: aws.String(userdata),
			TagSpecifications: []ec2Types.LaunchTemplateTagSpecificationRequest{
				{
					ResourceType: ec2Types.ResourceTypeInstance,
					Tags: []ec2Types.Tag{
						{
							Key:   aws.String(fmt.Sprintf(kubernetesTagFormat, clusterCfg.Metadata.Name)),
							Value: aws.String("owned"),
						},
					},
				},
			},
		},
	}

	if nodeGroup.SSH.PublicKeyName != nil {
		input.LaunchTemplateData.KeyName = nodeGroup.SSH.PublicKeyName
	}

	output, err := ec2Client.CreateLaunchTemplate(ctx, input)
	if err != nil {
		return "", errors.Wrap(err, "failed to create launch template")
	}

	return *output.LaunchTemplate.LaunchTemplateId, nil
}

func resolveAMI(ctx context.Context, ec2Client *ec2.Client, region, k8sMinorVersion, instanceType, amiFamily string) (string, error) {
	resolver := ami.NewAutoResolver(ec2Client)

	id, err := resolver.Resolve(ctx, region, k8sMinorVersion, instanceType, amiFamily)
	if err != nil {
		return "", errors.Wrap(err, "unable to determine AMI to use")
	}
	return id, nil
}

func deleteNodeGroup(ctx context.Context, eksClient *eks.Client, clusterName string) (string, string, error) {
	var notFoundErr *types.ResourceNotFoundException
	describeNGInput := &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(DefaultNodeGroupName),
	}
	ngInfo, err := eksClient.DescribeNodegroup(ctx, describeNGInput)
	if err != nil {
		if errors.As(err, &notFoundErr) {
			// the node group had already been deleted
			return "", "", nil
		} else {
			return "", "", errors.Wrapf(err, "failed to describe node group %s of cluster %s", DefaultNodeGroupName, clusterName)
		}
	}

	nodeGroupInput := &eks.DeleteNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(DefaultNodeGroupName),
	}
	_, err = eksClient.DeleteNodegroup(ctx, nodeGroupInput)
	if err != nil {
		return "", "", err
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-ticker.C:
			describeInput := &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(DefaultNodeGroupName),
			}
			_, err := eksClient.DescribeNodegroup(ctx, describeInput)
			if err != nil {
				if errors.As(err, &notFoundErr) {
					// the node group has already been deleted successfully
					return aws.ToString(ngInfo.Nodegroup.NodeRole), aws.ToString(ngInfo.Nodegroup.LaunchTemplate.Id), nil
				} else {
					return "", "", errors.Wrap(err, fmt.Sprintf("failed to describe node group %s of cluster %s", DefaultNodeGroupName, clusterName))
				}
			}
		}
	}
}

func deleteNodeLaunchTemplate(ctx context.Context, ec2Client *ec2.Client, launchTemplateId string) error {
	deleteLaunchTmplInput := &ec2.DeleteLaunchTemplateInput{
		LaunchTemplateId: aws.String(launchTemplateId),
	}
	_, err := ec2Client.DeleteLaunchTemplate(ctx, deleteLaunchTmplInput)
	if err != nil {
		return errors.Wrapf(err, "failed to delete node launch template %s", launchTemplateId)
	}
	return nil
}

func deleteCluster(ctx context.Context, eksClient *eks.Client, clusterName string) error {
	var notFoundErr *types.ResourceNotFoundException
	clusterInput := &eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	}
	_, err := eksClient.DeleteCluster(ctx, clusterInput)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			describeInput := &eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			}
			_, err := eksClient.DescribeCluster(ctx, describeInput)
			if err != nil {
				if errors.As(err, &notFoundErr) {
					// the cluster has already been deleted successfully
					return nil
				} else {
					return errors.Wrap(err, fmt.Sprintf("failed to describe EKS cluster %s to check delete progress", clusterName))
				}
			}
		}
	}
}
