package kutil

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type ListClusters struct {
	Region string
	Cloud  fi.Cloud
}

type ListClusterEntry struct {
	ClusterID string
}

func (c*ListClusters)  ListClusters() (map[string]*ListClusterEntry, error) {
	cloud := c.Cloud.(*fi.AWSCloud)

	clusters := make(map[string]*ListClusterEntry)

	{

		glog.V(2).Infof("Listing all EC2 tags matching cluster tags")
		var filters []*ec2.Filter
		filters = append(filters, fi.NewEC2Filter("key", "KubernetesCluster"))
		request := &ec2.DescribeTagsInput{
			Filters: filters,
		}
		response, err := cloud.EC2.DescribeTags(request)
		if err != nil {
			return nil, fmt.Errorf("error listing cluster tags: %v", err)
		}

		for _, t := range response.Tags {
			clusterID := *t.Value
			if clusters[clusterID] == nil {
				clusters[clusterID] = &ListClusterEntry{ClusterID: clusterID}
			}
		}
	}

	return clusters, nil
}
