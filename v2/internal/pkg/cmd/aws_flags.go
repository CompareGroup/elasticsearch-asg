package cmd

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"log"
	//"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	//"github.com/aws/aws-sdk-go-v2/aws/ec2metadata"
	//"github.com/aws/aws-sdk-go-v2/aws/external"
)

// AWSFlags represents a set of flags for connecting to AWS.
type AWSFlags struct {
	// Name of AWS region to use.
	Region string

	// Name of a shared AWS credentials profile to use.
	Profile string

	// Max number of retries to attempt on connection error.
	MaxRetries int

}

// NewAWSFlags returns a new BaseFlags.
func NewAWSFlags(app Flagger, maxRetries int) *AWSFlags {
	var f AWSFlags

	app.Flag("aws.region", "Name of AWS region to use.").
		PlaceHolder("REGION_NAME").
		StringVar(&f.Region)

	app.Flag("aws.profile", "Name of AWS credentials profile to use.").
		PlaceHolder("PROFILE_NAME").
		StringVar(&f.Profile)

	app.Flag("aws.max-retries", "Max number of retries to attempt on connection failure.").
		Hidden().
		Default(strconv.Itoa(maxRetries)).
		IntVar(&f.MaxRetries)

	return &f
}

// AWSConfig returns a aws.Config configure based on the default AWS
// config and these flags.
func (f *AWSFlags) AWSConfig(opts ...config.Config) aws.Config {
	if f.Region != "" {
		opts = append(opts, config.WithRegion(f.Region))
	}

	if f.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(f.Profile))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(f.Region),
		config.WithSharedConfigProfile(f.Profile),

		//config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKID", "SECRET_KEY", "TOKEN"))
	)
	//cfg, err := config.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		panic(any("unable to load AWS SDK default config, " + err.Error()))
	}

	if cfg.Region == "" {
		// Try setting region from EC2 metadata.
		metaClient := imds.NewFromConfig(cfg)
		region, err := metaClient.GetRegion(context.TODO(), &imds.GetRegionInput{})
		if err != nil {
			log.Printf("Unable to retrieve the region from the EC2 instance %v\n", err)
		}
		cfg.Region = region.Region
	}

	cfg.Retryer = func() aws.Retryer {
		return retry.AddWithMaxAttempts(retry.NewStandard(), f.MaxRetries)
	}

	return cfg
}
