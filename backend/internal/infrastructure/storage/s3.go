package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"lumio/internal/domain"
)

type S3 struct {
	client *s3.Client
	signer *s3.PresignClient
	bucket string
}

func New(ctx context.Context, endpoint, publicEndpoint, region, bucket, key, secret string) (*S3, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true })
	public := s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(publicEndpoint); o.UsePathStyle = true })
	return &S3{client: client, signer: s3.NewPresignClient(public), bucket: bucket}, nil
}
func (s *S3) UploadURL(ctx context.Context, key, kind string, size int64) (string, error) {
	r, err := s.signer.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, ContentType: &kind, ContentLength: &size, IfNoneMatch: aws.String("*")}, func(o *s3.PresignOptions) { o.Expires = 15 * time.Minute })
	if err != nil {
		return "", err
	}
	return r.URL, nil
}
func (s *S3) Check(ctx context.Context, key, kind string, size int64) error {
	r, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return fmt.Errorf("inspect upload: %w", err)
	}
	if aws.ToInt64(r.ContentLength) != size || aws.ToString(r.ContentType) != kind {
		return domain.ErrInvalid
	}
	return nil
}
func (s *S3) Download(ctx context.Context, key string, w io.Writer, limit int64) error {
	r, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return err
	}
	defer r.Body.Close()
	n, err := io.Copy(w, io.LimitReader(r.Body, limit+1))
	if err != nil {
		return err
	}
	if n != limit {
		return domain.ErrInvalid
	}
	return nil
}
func (s *S3) Put(ctx context.Context, key, kind string, r io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, ContentType: &kind, Body: r})
	return err
}
func (s *S3) PreviewURL(ctx context.Context, key string) (string, error) {
	r, err := s.signer.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key}, func(o *s3.PresignOptions) { o.Expires = 5 * time.Minute })
	if err != nil {
		return "", err
	}
	return r.URL, nil
}
func (s *S3) DeletePrefix(ctx context.Context, prefix string) error {
	return s.deleteObjects(ctx, prefix, "")
}
func (s *S3) PruneVariants(ctx context.Context, id, lease string) error {
	return s.deleteObjects(ctx, "media/"+id+"/v1/", "media/"+id+"/v1/"+lease+"/")
}
func (s *S3) deleteObjects(ctx context.Context, prefix, keep string) error {
	pages := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{Bucket: &s.bucket, Prefix: &prefix})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return err
		}
		if len(page.Contents) == 0 {
			continue
		}
		ids := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, o := range page.Contents {
			if keep == "" || !strings.HasPrefix(aws.ToString(o.Key), keep) {
				ids = append(ids, types.ObjectIdentifier{Key: o.Key})
			}
		}
		if len(ids) == 0 {
			continue
		}
		result, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: &s.bucket, Delete: &types.Delete{Objects: ids}})
		if err != nil {
			return err
		}
		if len(result.Errors) > 0 {
			return fmt.Errorf("delete objects: %s", aws.ToString(result.Errors[0].Code))
		}
	}
	return nil
}

// Initialize is an explicit local-development command, never run during server startup.
func (s *S3) Initialize(ctx context.Context) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.bucket})
	var owned *types.BucketAlreadyOwnedByYou
	if err != nil && !errors.As(err, &owned) {
		return err
	}
	if _, err = s.client.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{Bucket: &s.bucket}); err != nil {
		return err
	}
	_, err = s.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{Bucket: &s.bucket, LifecycleConfiguration: &types.BucketLifecycleConfiguration{Rules: []types.LifecycleRule{{ID: aws.String("expire-staged-uploads"), Status: types.ExpirationStatusEnabled, Filter: &types.LifecycleRuleFilter{Prefix: aws.String("uploads/")}, Expiration: &types.LifecycleExpiration{Days: aws.Int32(1)}}}}})
	return err
}

func (s *S3) Promote(ctx context.Context, source, destination string) error {
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{Bucket: &s.bucket, Key: &destination, CopySource: aws.String(url.PathEscape(s.bucket + "/" + source))})
	return err
}

func (s *S3) VerifyLifecycle(ctx context.Context) error {
	result, err := s.client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: &s.bucket})
	if err != nil {
		return fmt.Errorf("staging lifecycle is required: %w", err)
	}
	for _, rule := range result.Rules {
		if rule.Status == types.ExpirationStatusEnabled && rule.Filter != nil && aws.ToString(rule.Filter.Prefix) == "uploads/" && rule.Expiration != nil && aws.ToInt32(rule.Expiration.Days) == 1 {
			return nil
		}
	}
	return fmt.Errorf("bucket must expire uploads/ objects after one day")
}

func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, 0, err
	}
	return result.Body, aws.ToInt64(result.ContentLength), nil
}
