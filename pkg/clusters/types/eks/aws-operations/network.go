package awsoperations

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	defaultVPCCIDR           = "10.163.0.0/16"
	defaultSubnetCIDR1       = "10.163.1.0/24"
	defaultSubnetCIDR2       = "10.163.2.0/24"
	minimumAvailabilityZones = 2
)

func getAvailabilityZones(ctx context.Context, ec2Client *ec2.Client, region string) ([]string, error) {
	availabilityZonesOutput, err := ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe availability zones: %w", err)
	}
	var subnetAvZones []string
	for _, az := range availabilityZonesOutput.AvailabilityZones {
		if az.State == ec2Types.AvailabilityZoneStateAvailable && len(subnetAvZones) < 2 {
			subnetAvZones = append(subnetAvZones, *az.ZoneName)
		}
	}
	if len(subnetAvZones) < minimumAvailabilityZones {
		return nil, fmt.Errorf("there is no sufficient availability zones available in region %s: %w", region, err)
	}
	return subnetAvZones, nil
}

func createVPC(ctx context.Context, ec2Client *ec2.Client, subnetAvZones []string) (string, []string, error) {
	vpcOutput, err := ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String(defaultVPCCIDR),
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to create VPC: %w", err)
	}

	vpcID := *vpcOutput.Vpc.VpcId
	_, err = ec2Client.ModifyVpcAttribute(context.TODO(), &ec2.ModifyVpcAttributeInput{
		VpcId: aws.String(vpcID),
		EnableDnsSupport: &ec2Types.AttributeBooleanValue{
			Value: aws.Bool(true),
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to enable DNS support for VPC %s: %w", vpcID, err)
	}
	_, err = ec2Client.ModifyVpcAttribute(context.TODO(), &ec2.ModifyVpcAttributeInput{
		VpcId: aws.String(vpcID),
		EnableDnsHostnames: &ec2Types.AttributeBooleanValue{
			Value: aws.Bool(true),
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to enable DNS support for VPC %s: %w", vpcID, err)
	}

	igwOutput, err := ec2Client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{})
	if err != nil {
		return "", nil, fmt.Errorf("unable to create Internet Gateway: %w", err)
	}
	_, err = ec2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: igwOutput.InternetGateway.InternetGatewayId,
		VpcId:             vpcOutput.Vpc.VpcId,
	})
	if err != nil {
		return "", nil, fmt.Errorf("unable to add Internet Gateway %s within the VPC %s: %w", *igwOutput.InternetGateway.InternetGatewayId, vpcID, err)
	}
	rtOutput, err := ec2Client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: vpcOutput.Vpc.VpcId,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to create Route Table: %w", err)
	}
	_, err = ec2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         rtOutput.RouteTable.RouteTableId,
		GatewayId:            igwOutput.InternetGateway.InternetGatewayId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to create default egress route for Route Table %s: %w",
			*rtOutput.RouteTable.RouteTableId, err)
	}

	subnetID1, err := createSubnet(ctx, ec2Client, vpcID, defaultSubnetCIDR1, subnetAvZones[0], *rtOutput.RouteTable.RouteTableId)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create subnet within the VPC %s: %w", vpcID, err)
	}
	subnetID2, err := createSubnet(ctx, ec2Client, vpcID, defaultSubnetCIDR2, subnetAvZones[1], *rtOutput.RouteTable.RouteTableId)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create subnet within the VPC %s: %w", vpcID, err)
	}

	subnetIDs := []string{subnetID1, subnetID2}
	return vpcID, subnetIDs, nil
}

func createSubnet(ctx context.Context, ec2Client *ec2.Client, vpcID, cidrBlock, availabilityZone, routeTableID string) (string, error) {
	subnet1Output, err := ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcID),
		CidrBlock:        aws.String(cidrBlock),
		AvailabilityZone: aws.String(availabilityZone),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create subnet within the VPC %s: %w", vpcID, err)
	}

	subnetID := subnet1Output.Subnet.SubnetId
	_, err = ec2Client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
		SubnetId:            subnetID,
		MapPublicIpOnLaunch: &ec2Types.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	if err != nil {
		return "", fmt.Errorf("unable to modify subnet %s to enable public IP assignment: %w", *subnetID, err)
	}

	if routeTableID != "" {
		_, err = ec2Client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(routeTableID),
			SubnetId:     subnetID,
		})
		if err != nil {
			return "", fmt.Errorf("failed to associate Route Table %s with subnet %s: %w", routeTableID, *subnetID, err)
		}
	}
	return *subnetID, nil
}

func createControlPlaneSecurityGroup(ctx context.Context, ec2Client *ec2.Client, vpcID, clusterName string) (string, error) {
	sg1Output, err := ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-cp", clusterName)),
		Description: aws.String("Allow communication between the control plane and worker nodes"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []ec2Types.TagSpecification{
			{
				ResourceType: ec2Types.ResourceTypeSecurityGroup,
				Tags: []ec2Types.Tag{
					{
						Key:   aws.String(fmt.Sprintf(kubernetesTagFormat, clusterName)),
						Value: aws.String("owned"),
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %w", err)
	}
	return *sg1Output.GroupId, nil
}

func createNodeSecurityGroup(ctx context.Context, ec2Client *ec2.Client, vpcID, clusterName string, cpDefaultSecurityGroupIDs []string) (string, error) {
	sgOutput, err := ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-shared-by-all-nodes", clusterName)),
		Description: aws.String("Allow communication between all nodes in the cluster"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []ec2Types.TagSpecification{
			{
				ResourceType: ec2Types.ResourceTypeSecurityGroup,
				Tags: []ec2Types.Tag{
					{
						Key:   aws.String(fmt.Sprintf(kubernetesTagFormat, clusterName)),
						Value: aws.String("owned"),
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create node security group: %w", err)
	}

	for _, sgID := range cpDefaultSecurityGroupIDs {
		_, err = ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId: sgOutput.GroupId,
			IpPermissions: []ec2Types.IpPermission{
				{
					IpProtocol: aws.String("-1"),
					UserIdGroupPairs: []ec2Types.UserIdGroupPair{
						{
							GroupId: aws.String(sgID),
						},
					},
				},
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to authorize inbound traffic from control plane security group %s to node security group %s: %w",
				sgID, *sgOutput.GroupId, err)
		}

		_, err = ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId: aws.String(sgID),
			IpPermissions: []ec2Types.IpPermission{
				{
					IpProtocol: aws.String("-1"),
					UserIdGroupPairs: []ec2Types.UserIdGroupPair{
						{
							GroupId: sgOutput.GroupId,
						},
					},
				},
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to authorize inbound traffic from node security group %s to control plane security group %s: %w",
				*sgOutput.GroupId, sgID, err)
		}
	}

	return *sgOutput.GroupId, nil
}

func deleteVPC(ctx context.Context, ec2Client *ec2.Client, vpcID string) error {
	routeTablesOutput, err := ec2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2Types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list route tables in VPC %s: %w", vpcID, err)
	}

	for _, rt := range routeTablesOutput.RouteTables {
		isMain := false
		for _, assoc := range rt.Associations {
			if assoc.Main != nil && *assoc.Main {
				isMain = true
				break
			}
		}
		if isMain {
			continue
		}

		for _, assoc := range rt.Associations {
			if assoc.RouteTableAssociationId != nil {
				_, err := ec2Client.DisassociateRouteTable(ctx, &ec2.DisassociateRouteTableInput{
					AssociationId: assoc.RouteTableAssociationId,
				})
				if err != nil {
					return fmt.Errorf("failed to disassociate route table association %s for route table %s: %w",
						*assoc.RouteTableAssociationId, *rt.RouteTableId, err)
				}
			}
		}

		_, err := ec2Client.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
			RouteTableId: rt.RouteTableId,
		})
		if err != nil {
			return fmt.Errorf("failed to delete route table %s: %w", *rt.RouteTableId, err)
		}
	}

	subnetsOutput, err := ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2Types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe subnets in VPC %s: %w", vpcID, err)
	}

	for _, subnet := range subnetsOutput.Subnets {
		_, err := ec2Client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		})
		if err != nil {
			return fmt.Errorf("failed to delete subnet %s: %w", *subnet.SubnetId, err)
		}
	}

	igwsOutput, err := ec2Client.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
		Filters: []ec2Types.Filter{
			{Name: aws.String("attachment.vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe internet gateways in VPC %s: %w", vpcID, err)
	}

	for _, igw := range igwsOutput.InternetGateways {
		_, err := ec2Client.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
			VpcId:             aws.String(vpcID),
		})
		if err != nil {
			return fmt.Errorf("failed to detach internet gateway %s: %w", *igw.InternetGatewayId, err)
		}

		_, err = ec2Client.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
		})
		if err != nil {
			return fmt.Errorf("failed to delete internet gateway %s: %w", *igw.InternetGatewayId, err)
		}
	}

	sgOutput, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2Types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe security groups in VPC %s: %w", vpcID, err)
	}

	for _, sg := range sgOutput.SecurityGroups {
		if sg.GroupName != nil && *sg.GroupName == "default" {
			continue
		}

		for _, ingress := range sg.IpPermissions {
			_, err := ec2Client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
				GroupId:       sg.GroupId,
				IpPermissions: []ec2Types.IpPermission{ingress},
			})
			if err != nil {
				return fmt.Errorf("failed to revoke a %s ingress rule on security group %s: %w",
					aws.ToString(ingress.IpProtocol), aws.ToString(sg.GroupId), err)
			}
		}

		for _, egress := range sg.IpPermissionsEgress {
			_, err := ec2Client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
				GroupId:       sg.GroupId,
				IpPermissions: []ec2Types.IpPermission{egress},
			})
			if err != nil {
				return fmt.Errorf("failed to revoke a %s egress rule on security group %s: %w",
					aws.ToString(egress.IpProtocol), aws.ToString(sg.GroupId), err)
			}
		}
	}

	for _, sg := range sgOutput.SecurityGroups {
		if sg.GroupName != nil && *sg.GroupName == "default" {
			continue
		}

		_, err := ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: sg.GroupId,
		})
		if err != nil {
			return fmt.Errorf("failed to delete security group %s: %w", *sg.GroupId, err)
		}
	}

	_, err = ec2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete VPC %s: %w", vpcID, err)
	}

	return nil
}
