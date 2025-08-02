package aws

import (
	"fmt"
)

// Region represents an AWS region with its endpoint information
type Region struct {
	ID       string
	Name     string
	Endpoint string
}

// GetRegions returns a list of AWS regions with their EC2 ping endpoints
func GetRegions() []Region {
	return []Region{
		{ID: "ap-northeast-1", Name: "Asia Pacific (Tokyo)", Endpoint: "https://ec2.ap-northeast-1.amazonaws.com/ping"},
		{ID: "ap-northeast-2", Name: "Asia Pacific (Seoul)", Endpoint: "https://ec2.ap-northeast-2.amazonaws.com/ping"},
		{ID: "ap-northeast-3", Name: "Asia Pacific (Osaka)", Endpoint: "https://ec2.ap-northeast-3.amazonaws.com/ping"},
		{ID: "ap-south-1", Name: "Asia Pacific (Mumbai)", Endpoint: "https://ec2.ap-south-1.amazonaws.com/ping"},
		{ID: "ap-southeast-1", Name: "Asia Pacific (Singapore)", Endpoint: "https://ec2.ap-southeast-1.amazonaws.com/ping"},
		{ID: "ap-southeast-2", Name: "Asia Pacific (Sydney)", Endpoint: "https://ec2.ap-southeast-2.amazonaws.com/ping"},
		{ID: "ca-central-1", Name: "Canada (Central)", Endpoint: "https://ec2.ca-central-1.amazonaws.com/ping"},
		{ID: "eu-central-1", Name: "Europe (Frankfurt)", Endpoint: "https://ec2.eu-central-1.amazonaws.com/ping"},
		{ID: "eu-north-1", Name: "Europe (Stockholm)", Endpoint: "https://ec2.eu-north-1.amazonaws.com/ping"},
		{ID: "eu-west-1", Name: "Europe (Ireland)", Endpoint: "https://ec2.eu-west-1.amazonaws.com/ping"},
		{ID: "eu-west-2", Name: "Europe (London)", Endpoint: "https://ec2.eu-west-2.amazonaws.com/ping"},
		{ID: "eu-west-3", Name: "Europe (Paris)", Endpoint: "https://ec2.eu-west-3.amazonaws.com/ping"},
		{ID: "sa-east-1", Name: "South America (São Paulo)", Endpoint: "https://ec2.sa-east-1.amazonaws.com/ping"},
		{ID: "us-east-1", Name: "US East (N. Virginia)", Endpoint: "https://ec2.us-east-1.amazonaws.com/ping"},
		{ID: "us-east-2", Name: "US East (Ohio)", Endpoint: "https://ec2.us-east-2.amazonaws.com/ping"},
		{ID: "us-west-1", Name: "US West (N. California)", Endpoint: "https://ec2.us-west-1.amazonaws.com/ping"},
		{ID: "us-west-2", Name: "US West (Oregon)", Endpoint: "https://ec2.us-west-2.amazonaws.com/ping"},
	}
}

// GetEC2Endpoint returns the EC2 ping endpoint URL for a given region ID
func GetEC2Endpoint(regionID string) string {
	return fmt.Sprintf("https://ec2.%s.amazonaws.com/ping", regionID)
}