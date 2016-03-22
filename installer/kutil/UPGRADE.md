
To upgrade from 1.1 to 1.2

go install k8s.io/contrib/installer/kutil

# Set the ZONE env var to the zone where your cluster is
ZONE=us-east-1c

# Set bucket to the name of your S3 bucket for artifacts
BUCKET=<somes3bucket>

# List all the clusters in ${ZONE}
kutil get cluster --zone ${ZONE}

# The first column is your cluster ID (likely `kubernetes`)
# The second column is your master IP, and should match the output of `kubectl cluster-info`

# You will also need the IP address of one of your nodes
kubectl get nodes -ojson
# Extract out one of the 'ExternalIP' values

kutil export cluster --master 52.201.246.236 -i ~/.ssh/kube_aws_rsa --logtostderr --node 52.90.58.244 --dest upgrade11/

# This should have extracted keys & configuration from the running cluster:
find upgrade11/
# should show ....
upgrade11/
upgrade11/pki
upgrade11/pki/ca.crt
upgrade11/pki/issued
upgrade11/pki/issued/cn=kubernetes-master.crt
upgrade11/pki/issued/cn=kubelet.crt
upgrade11/pki/private
upgrade11/pki/private/cn=kubernetes-master.key
upgrade11/pki/private/cn=kubelet.key
upgrade11/kubernetes.yaml



# kubernetes.yaml has your configuration
> cat upgrade11/kubernetes.yaml
AllocateNodeCIDRs: true
CloudProvider: aws
ClusterID: kubernetes
ClusterIPRange: 10.244.0.0/16
DNSDomain: cluster.local
DNSReplicas: 1
DNSServerIP: 10.0.0.10
DockerStorage: aufs
ElasticsearchLoggingReplicas: 1
EnableClusterDNS: true
EnableClusterLogging: true
EnableClusterMonitoring: influxdb
EnableClusterUI: true
EnableCustomMetrics: false
EnableNodeLogging: true
InternetGatewayID: igw-db10afbf
KubePassword: PTajI3M5mEdoicTo
KubeProxyToken: TraE1N4igFN8gChk65LT1S3VhWea2mwr
KubeUser: admin
KubeletToken: QK5ozLccTx5OshOAcZEVYz1TDOsP9NR1
LoggingDestination: elasticsearch
MasterIPRange: 10.246.0.0/24
NodeCount: 4
RouteTableID: rtb-2c1dc94b
ServiceClusterIPRange: 10.0.0.0/16
SubnetID: subnet-d32547a5
VPCID: vpc-d9080cbd
Zone: us-east-1c

# Download the kubernetes install that you are going to install
release=v1.2.0
mkdir release-${release}
wget https://storage.googleapis.com/kubernetes-release/release/${release}/kubernetes.tar.gz -O release-${release}/kubernetes.tar.gz
tar zxf  release-${release}/kubernetes.tar.gz -C  release-${release}


kutil create cluster -i ~/.ssh/id_rsa -d upgrade11/ -r release-${release}/kubernetes/ --logtostderr --s3-bucket ${BUCKET} -t bash

Should output a bash script that could be used to reconfigure the cluster:
(note that it doesn't currently enforce the image change for the 1.2 uprgade):

#!/bin/bash
set -ex

. ./helpers

export AWS_DEFAULT_REGION="us-east-1"
export AWS_DEFAULT_OUTPUT="text"
PERSISTENTVOLUME_1="vol-884ebb20"
ELASTICIP_1="eipalloc-3fe11c58"
IAMROLE_1="AROAILHQRUIC4GMMJFPZS"
IAMROLE_2="AROAJQYAOWIFBGC7M4J64"
VPC_1="vpc-d9080cbd"
add-tag ${VPC_1} "Name" "kubernetes-kubernetes"
DHCPOPTIONS_1="dopt-f7f9a592"
SUBNET_1="subnet-d32547a5"
add-tag ${SUBNET_1} "Name" "kubernetes-kubernetes"
INTERNETGATEWAY_1="igw-db10afbf"
add-tag ${INTERNETGATEWAY_1} "Name" "kubernetes-kubernetes"
add-tag ${INTERNETGATEWAY_1} "KubernetesCluster" "kubernetes"
ROUTETABLE_1="rtb-2c1dc94b"
add-tag ${ROUTETABLE_1} "Name" "kubernetes-kubernetes"
ROUTETABLEASSOCIATION_1="rtbassoc-7eab9719"
SECURITYGROUP_1="sg-d1bb23a9"
add-tag ${SECURITYGROUP_1} "Name" "kubernetes-master-kubernetes"
SECURITYGROUP_2="sg-d5bb23ad"
add-tag ${SECURITYGROUP_2} "Name" "kubernetes-minion-kubernetes"
INSTANCE_1="i-434d1ec7"
add-tag ${INSTANCE_1} "Role" "master"


There should be nothing other than tags in the bash scripts output.  If there is, stop and open an issue!

Now comes the moment of truth.


# Shut down your master:
aws ec2 terminate-instances --instance-id i-434d1ec7

# Run in bash mode again to see the changes it will make:
kutil create cluster -i ~/.ssh/id_rsa -d upgrade11/ -r release-${release}/kubernetes/ --logtostderr --s3-bucket ${BUCKET} -t bash


# Now you can either execute that bash script, or have kutil run directly:
kutil create cluster -i ~/.ssh/id_rsa -d upgrade11/ -r release-${release}/kubernetes/ --logtostderr --s3-bucket ${BUCKET} -t direct

# Now once again list your clusters; if you weren't using an elastic IP previous one will have been allocated
kutil get cluster --zone ${ZONE}

# Now, if the IP address changed, this means your kubecfg is now pointing to an invalid IP
# The easiest way is to go into your ~/.kube/config file and change the IP
# But you can also do:
#kutil create kubecfg --master 52.87.136.167



ssh admin@52.87.136.167 mkdir /tmp/ca
scp upgrade11/pki/issued/cn\=kubernetes-master.crt  admin@52.87.136.167:/tmp/ca/server.cert
scp upgrade11/pki/private/cn\=kubernetes-master.key admin@52.87.136.167:/tmp/ca/server.key
scp upgrade11/pki/issued/cn\=kubecfg.crt  admin@52.87.136.167:/tmp/ca/kubecfg.crt
scp upgrade11/pki/private/cn\=kubecfg.key admin@52.87.136.167:/tmp/ca/kubecfg.key
ssh admin@52.87.136.167 sudo cp /tmp/ca/* /mnt/master-pd/srv/kubernetes/
ssh admin@52.87.136.167 sudo chown root:root /mnt/master-pd/srv/kubernetes/ca.crt /mnt/master-pd/srv/kubernetes/server.* /mnt/master-pd/srv/kubernetes/kubecfg.*
ssh admin@52.87.136.167 sudo chmod 600  /mnt/master-pd/srv/kubernetes/ca.crt /mnt/master-pd/srv/kubernetes/server.* /mnt/master-pd/srv/kubernetes/kubecfg.*
ssh admin@52.87.136.167 rm -rf /tmp/ca
ssh admin@52.87.136.167 sudo systemctl restart docker


kutil create kubecfg --master 52.87.136.167  -i ~/.ssh/id_rsa



