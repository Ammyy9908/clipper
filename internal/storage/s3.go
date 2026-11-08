package storage

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/ammyy9908/clipper/internal/config"
)

type Store struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func New(ctx context.Context, cfg config.Config) (*Store, error) {
	base, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithRegion(cfg.S3Region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(base, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = true // MinIO / R2
		}
	})
	return &Store{client: client, presign: s3.NewPresignClient(client), bucket: cfg.S3Bucket}, nil
}

// EnsureLifecycle sets an automated expiration policy on the bucket for outputs/ and uploads/
// so Cloudflare R2 automatically purges old files without requiring manual deletion.
func (s *Store) EnsureLifecycle(ctx context.Context, days int32) error {
	if days < 1 {
		days = 1
	}
	_, err := s.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(s.bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("expire-outputs"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilterMemberPrefix{Value: "outputs/"},
					Expiration: &types.LifecycleExpiration{
						Days: aws.Int32(days),
					},
					AbortIncompleteMultipartUpload: &types.AbortIncompleteMultipartUpload{
						DaysAfterInitiation: aws.Int32(1),
					},
				},
				{
					ID:     aws.String("expire-uploads"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilterMemberPrefix{Value: "uploads/"},
					Expiration: &types.LifecycleExpiration{
						Days: aws.Int32(days),
					},
					AbortIncompleteMultipartUpload: &types.AbortIncompleteMultipartUpload{
						DaysAfterInitiation: aws.Int32(1),
					},
				},
				{
					ID:     aws.String("expire-sources"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilterMemberPrefix{Value: "sources/"},
					Expiration: &types.LifecycleExpiration{
						Days: aws.Int32(days),
					},
					AbortIncompleteMultipartUpload: &types.AbortIncompleteMultipartUpload{
						DaysAfterInitiation: aws.Int32(1),
					},
				},
			},
		},
	})
	return err
}

func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *Store) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Download streams an object to a local path. Workers need the file on disk
// because ffmpeg seeks; piping a stream in defeats -c copy segmenting.
func (s *Store) Download(ctx context.Context, key, dest string) error {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get %s: %w", key, err)
	}
	defer out.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.ReadFrom(out.Body)
	return err
}

func (s *Store) Upload(ctx context.Context, key, src, contentType string) (int64, error) {
	f, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          f,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(st.Size()),
	})
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func (s *Store) HeadSize(ctx context.Context, key string) (int64, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, err
	}
	if out.ContentLength == nil {
		return 0, nil
	}
	return *out.ContentLength, nil
}
