package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Store implements the Store interface using S3 as backend
type S3Store struct {
	client *s3.Client
	bucket string
}

func NewS3Store(bucket string) (*S3Store, error) {
	ctx := context.Background()

	var cfg aws.Config
	var err error

	endpoint := os.Getenv("S3_ENDPOINT")
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-west-2" // fallback
	}

	// Debug logs
	println("[DEBUG] Using AWS_REGION =", region)
	println("[DEBUG] Using S3_ENDPOINT =", endpoint)

	if endpoint != "" {
		// LocalStack mode
		cfg, err = config.LoadDefaultConfig(
			ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
			config.WithEndpointResolverWithOptions(
				aws.EndpointResolverWithOptionsFunc(
					func(service, region string, options ...interface{}) (aws.Endpoint, error) {
						if service == s3.ServiceID {
							return aws.Endpoint{
								URL:               endpoint,
								HostnameImmutable: true,
							}, nil
						}
						return aws.Endpoint{}, &aws.EndpointNotFoundError{}
					},
				),
			),
		)
	} else {
		// Real AWS (critical fix: Must supply region explicitly)
		cfg, err = config.LoadDefaultConfig(
			ctx,
			config.WithRegion(region),
		)
	}

	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)
	store := &S3Store{
		client: client,
		bucket: bucket,
	}

	// Try to create bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		},
	})
	if err != nil {
		var be *types.BucketAlreadyOwnedByYou
		if !errors.As(err, &be) {
			return nil, err
		}
	}

	return store, nil
}

// Write to S3
func (s *S3Store) Set(key, value string, version *int) (int, error) {
	ctx := context.Background()

	// calculate version: if leader writes (version == nil), based on existing version +1
	var v int
	if version == nil {
		old, exists, err := s.Get(key)
		if err != nil {
			return 0, err
		}
		if exists {
			v = old.Version + 1
		} else {
			v = 1
		}
	} else {
		// follower replication uses given version
		v = *version
	}

	obj := KVPair{
		Value:   value,
		Version: v,
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return 0, err
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return 0, err
	}

	return v, nil
}

// Read from S3
func (s *S3Store) Get(key string) (KVPair, bool, error) {
	ctx := context.Background()

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return KVPair{}, false, nil
		}
		return KVPair{}, false, err
	}
	defer out.Body.Close()

	b, err := io.ReadAll(out.Body)
	if err != nil {
		return KVPair{}, false, err
	}

	var obj KVPair
	if err := json.Unmarshal(b, &obj); err != nil {
		return KVPair{}, false, err
	}

	return obj, true, nil
}
