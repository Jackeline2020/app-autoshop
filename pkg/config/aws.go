package config

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

// LoadAWSConfig carrega as credenciais da AWS do ambiente. Localmente,
// AWS_ENDPOINT_URL aponta para o LocalStack.
func LoadAWSConfig(ctx context.Context) (aws.Config, error) {
	return awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(Env("AWS_REGION", "us-east-1")))
}
