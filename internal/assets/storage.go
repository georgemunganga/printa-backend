// Package assets stores customer-owned design files. S3 is selected only when its
// complete configuration is present; PostgreSQL bytea storage is a development fallback.
package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

const MaxSize = 20 << 20

type Asset struct {
	ID, OwnerID, Name, ContentType string
	Size                           int64
	Provider                       string
	Key                            string
	Content                        []byte
}
type Storage interface {
	Upload(context.Context, string, string, string, []byte) (*Asset, error)
	Open(context.Context, string, string) (*Asset, error)
}

func NewStorage(db *sql.DB) (Storage, error) {
	endpoint, bucket, region, key, secret := strings.TrimSpace(os.Getenv("AWS_S3_ENDPOINT")), strings.TrimSpace(os.Getenv("AWS_S3_BUCKET")), strings.TrimSpace(os.Getenv("AWS_S3_REGION")), strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID")), strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if endpoint == "" && bucket == "" && region == "" && key == "" && secret == "" {
		return &databaseStorage{db: db}, nil
	}
	if endpoint == "" || bucket == "" || region == "" || key == "" || secret == "" {
		return nil, errors.New("S3 configuration is incomplete; set AWS_S3_ENDPOINT, AWS_S3_BUCKET, AWS_S3_REGION, AWS_ACCESS_KEY_ID, and AWS_SECRET_ACCESS_KEY, or clear all to use the PostgreSQL development fallback")
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")), config.WithBaseEndpoint(endpoint))
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	return &s3Storage{db: db, bucket: bucket, client: s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })}, nil
}

type databaseStorage struct{ db *sql.DB }

func (s *databaseStorage) Upload(ctx context.Context, owner, name, contentType string, data []byte) (*Asset, error) {
	return save(ctx, s.db, owner, name, contentType, data, "DATABASE", "")
}
func (s *databaseStorage) Open(ctx context.Context, id, owner string) (*Asset, error) {
	return load(ctx, s.db, id, owner, true)
}

type s3Storage struct {
	db     *sql.DB
	bucket string
	client *s3.Client
}

func (s *s3Storage) Upload(ctx context.Context, owner, name, contentType string, data []byte) (*Asset, error) {
	id := uuid.NewString()
	key := fmt.Sprintf("design-assets/%s/%s/%s", owner, id, path.Base(name))
	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String(contentType), ServerSideEncryption: types.ServerSideEncryptionAes256}); err != nil {
		return nil, fmt.Errorf("upload S3 object: %w", err)
	}
	return saveWithID(ctx, s.db, id, owner, name, contentType, data, "S3", key)
}
func (s *s3Storage) Open(ctx context.Context, id, owner string) (*Asset, error) {
	a, err := load(ctx, s.db, id, owner, false)
	if err != nil {
		return nil, err
	}
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(a.Key)})
	if err != nil {
		return nil, fmt.Errorf("retrieve S3 object: %w", err)
	}
	defer object.Body.Close()
	a.Content, err = io.ReadAll(io.LimitReader(object.Body, MaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read S3 object: %w", err)
	}
	if len(a.Content) > MaxSize {
		return nil, errors.New("stored object exceeds allowed design asset size")
	}
	return a, nil
}

func save(ctx context.Context, db *sql.DB, owner, name, contentType string, data []byte, provider, key string) (*Asset, error) {
	return saveWithID(ctx, db, uuid.NewString(), owner, name, contentType, data, provider, key)
}
func saveWithID(ctx context.Context, db *sql.DB, id, owner, name, contentType string, data []byte, provider, key string) (*Asset, error) {
	if owner == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(contentType) == "" {
		return nil, errors.New("owner, file name, and content type are required")
	}
	if len(data) == 0 || len(data) > MaxSize {
		return nil, fmt.Errorf("file must be between 1 byte and %d bytes", MaxSize)
	}
	hash := sha256.Sum256(data)
	var content interface{} = data
	if provider == "S3" {
		content = nil
	}
	_, err := db.ExecContext(ctx, `INSERT INTO design_assets (id, owner_id, original_name, content_type, size_bytes, storage_provider, storage_key, content, checksum_sha256) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9)`, id, owner, path.Base(name), contentType, len(data), provider, key, content, hex.EncodeToString(hash[:]))
	if err != nil {
		return nil, fmt.Errorf("record design asset: %w", err)
	}
	return &Asset{ID: id, OwnerID: owner, Name: path.Base(name), ContentType: contentType, Size: int64(len(data)), Provider: provider, Key: key}, nil
}
func load(ctx context.Context, db *sql.DB, id, owner string, includeContent bool) (*Asset, error) {
	a := &Asset{}
	var content []byte
	var key sql.NullString
	query := `SELECT id, owner_id, original_name, content_type, size_bytes, storage_provider, storage_key, content FROM design_assets WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL`
	if err := db.QueryRowContext(ctx, query, id, owner).Scan(&a.ID, &a.OwnerID, &a.Name, &a.ContentType, &a.Size, &a.Provider, &key, &content); err != nil {
		return nil, err
	}
	a.Key = key.String
	if includeContent {
		a.Content = content
	}
	return a, nil
}

var _ = io.EOF
var _ = time.Now
