/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testutils

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/golang/glog"
	kopsroot "k8s.io/kops"
	"k8s.io/kops/cloudmock/aws/mockec2"
	"k8s.io/kops/cloudmock/aws/mockelb"
	"k8s.io/kops/cloudmock/aws/mockroute53"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
	"k8s.io/kops/upup/pkg/fi/cloudup/gce"
	"k8s.io/kops/util/pkg/vfs"
)

type IntegrationTestHarness struct {
	TempDir string
	T       *testing.T

	// The original kops DefaultChannelBase value, restored on Close
	originalDefaultChannelBase string

	// originalKopsVersion is the original kops.Version value, restored on Close
	originalKopsVersion string
}

func NewIntegrationTestHarness(t *testing.T) *IntegrationTestHarness {
	h := &IntegrationTestHarness{}
	tempDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	h.TempDir = tempDir

	vfs.Context.ResetMemfsContext(true)

	// Replace the default channel path with a local filesystem path, so we don't try to retrieve it from a server
	{
		channelPath, err := filepath.Abs(path.Join("../../channels/"))
		if err != nil {
			t.Fatalf("error resolving stable channel path: %v", err)
		}
		channelPath += "/"
		h.originalDefaultChannelBase = kops.DefaultChannelBase
		kops.DefaultChannelBase = "file://" + channelPath
	}

	return h
}

func (h *IntegrationTestHarness) Close() {
	if h.TempDir != "" {
		if os.Getenv("KEEP_TEMP_DIR") != "" {
			glog.Infof("NOT removing temp directory, because KEEP_TEMP_DIR is set: %s", h.TempDir)
		} else {
			err := os.RemoveAll(h.TempDir)
			if err != nil {
				h.T.Fatalf("failed to remove temp dir %q: %v", h.TempDir, err)
			}
		}
	}

	if h.originalKopsVersion != "" {
		kopsroot.Version = h.originalKopsVersion
	}

	if h.originalDefaultChannelBase != "" {
		kops.DefaultChannelBase = h.originalDefaultChannelBase
	}
}

func (h *IntegrationTestHarness) SetupMockAWS() {
	cloud := awsup.InstallMockAWSCloud("us-test-1", "abc")
	mockEC2 := &mockec2.MockEC2{}
	cloud.MockEC2 = mockEC2
	mockRoute53 := &mockroute53.MockRoute53{}
	cloud.MockRoute53 = mockRoute53
	mockELB := &mockelb.MockELB{}
	cloud.MockELB = mockELB

	mockRoute53.MockCreateZone(&route53.HostedZone{
		Id:   aws.String("/hostedzone/Z1AFAKE1ZON3YO"),
		Name: aws.String("example.com."),
		Config: &route53.HostedZoneConfig{
			PrivateZone: aws.Bool(false),
		},
	}, nil)
	mockRoute53.MockCreateZone(&route53.HostedZone{
		Id:   aws.String("/hostedzone/Z2AFAKE1ZON3NO"),
		Name: aws.String("internal.example.com."),
		Config: &route53.HostedZoneConfig{
			PrivateZone: aws.Bool(true),
		},
	}, []*route53.VPC{{
		VPCId: aws.String("vpc-23456789"),
	}})
	mockRoute53.MockCreateZone(&route53.HostedZone{
		Id:   aws.String("/hostedzone/Z3AFAKE1ZOMORE"),
		Name: aws.String("private.example.com."),
		Config: &route53.HostedZoneConfig{
			PrivateZone: aws.Bool(true),
		},
	}, []*route53.VPC{{
		VPCId: aws.String("vpc-12345678"),
	}})

	mockEC2.Images = append(mockEC2.Images, &ec2.Image{
		ImageId:        aws.String("ami-12345678"),
		Name:           aws.String("k8s-1.4-debian-jessie-amd64-hvm-ebs-2016-10-21"),
		OwnerId:        aws.String(awsup.WellKnownAccountKopeio),
		RootDeviceName: aws.String("/dev/xvda"),
	})

	mockEC2.Images = append(mockEC2.Images, &ec2.Image{
		ImageId:        aws.String("ami-15000000"),
		Name:           aws.String("k8s-1.5-debian-jessie-amd64-hvm-ebs-2017-01-09"),
		OwnerId:        aws.String(awsup.WellKnownAccountKopeio),
		RootDeviceName: aws.String("/dev/xvda"),
	})
	mockEC2.CreateInternetGateway(&ec2.CreateInternetGatewayInput{})
	mockEC2.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String("igw-1"),
		VpcId:             aws.String("vpc-12345678"),
	})
}

// SetupMockGCE configures a mock GCE cloud provider
func (h *IntegrationTestHarness) SetupMockGCE() {
	gce.InstallMockGCECloud("us-test1", "testproject")
}

// MockKopsVersion will set the kops version to the specified value, until Close is called
func (h *IntegrationTestHarness) MockKopsVersion(version string) {
	if h.originalKopsVersion != "" {
		h.T.Fatalf("MockKopsVersion called twice (%s and %s)", version, h.originalKopsVersion)
	}

	h.originalKopsVersion = kopsroot.Version
	kopsroot.Version = version
}
