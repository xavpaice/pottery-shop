package r2

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"pottery-shop/cmd/cardboard/internal/config"
)

// Track represents a single MP3 track exposed by the R2 store.
type Track struct {
	Key          string    `json:"key"`
	Title        string    `json:"title"`
	Artist       string    `json:"artist"`
	Album        string    `json:"album"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
	URL          string    `json:"url"`
	DownloadURL  string    `json:"downloadUrl"`
}

// Store wraps an S3-compatible client pointing at a Cloudflare R2 bucket.
type Store struct {
	cfg         *config.Config
	client      *s3.Client
	presignClient *s3.PresignClient
}

// NewStore creates an S3 client configured for R2 using the provided config.
func NewStore(cfg *config.Config) (*Store, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.R2Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.R2AccessKeyID,
				cfg.R2SecretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.R2Endpoint)
		o.UsePathStyle = true
	})

	return &Store{
		cfg:           cfg,
		client:        client,
		presignClient: s3.NewPresignClient(client),
	}, nil
}

// Ready checks connectivity by sending a HeadBucket request.
func (s *Store) Ready(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.cfg.R2Bucket),
	})
	return err
}

// ListTracks returns all MP3 objects in the bucket enriched with metadata and URLs.
func (s *Store) ListTracks(ctx context.Context) ([]Track, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.R2Bucket),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			key := *obj.Key
			if strings.HasSuffix(strings.ToLower(key), ".mp3") {
				keys = append(keys, key)
			}
		}
	}

	tracks, err := s.enrichTracks(ctx, keys)
	if err != nil {
		return nil, err
	}

	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].Key < tracks[j].Key
	})
	return tracks, nil
}

// enrichTracks runs HeadObject for each key in parallel using a worker pool.
func (s *Store) enrichTracks(ctx context.Context, keys []string) ([]Track, error) {
	const workers = 10
	type result struct {
		track Track
		err   error
	}

	keyCh := make(chan string)
	resCh := make(chan result, len(keys))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range keyCh {
				track, err := s.headTrack(ctx, key)
				resCh <- result{track: track, err: err}
			}
		}()
	}

	go func() {
		for _, key := range keys {
			keyCh <- key
		}
		close(keyCh)
	}()

	go func() {
		wg.Wait()
		close(resCh)
	}()

	tracks := make([]Track, 0, len(keys))
	var firstErr error
	for r := range resCh {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		if r.track.Key != "" {
			tracks = append(tracks, r.track)
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return tracks, nil
}

func (s *Store) headTrack(ctx context.Context, key string) (Track, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.R2Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Track{}, fmt.Errorf("head object %q: %w", key, err)
	}

	meta := normalizeMetadata(out.Metadata)
	artist, title, album := deriveTrackInfo(key, meta)

	publicURL := buildPublicURL(s.cfg.R2PublicURLPrefix, key)

	presignOut, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.R2Bucket),
		Key:    aws.String(key),
		ResponseContentDisposition: aws.String(
			fmt.Sprintf("attachment; filename=%q", filepath.Base(key)),
		),
	}, s3.WithPresignExpires(s.cfg.R2PresignTTL))
	if err != nil {
		return Track{}, fmt.Errorf("presign object %q: %w", key, err)
	}

	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	var lastMod time.Time
	if out.LastModified != nil {
		lastMod = *out.LastModified
	}

	return Track{
		Key:          key,
		Title:        title,
		Artist:       artist,
		Album:        album,
		Size:         size,
		LastModified: lastMod,
		URL:          publicURL,
		DownloadURL:  presignOut.URL,
	}, nil
}

// buildPublicURL joins the R2 public prefix and object key.
func buildPublicURL(prefix, key string) string {
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(key, "/")
}

// normalizeMetadata lowercases all metadata keys so lookups are case-insensitive.
func normalizeMetadata(meta map[string]string) map[string]string {
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		out[strings.ToLower(k)] = v
	}
	return out
}

// deriveTrackInfo returns artist, title, and album from metadata or the key path.
func deriveTrackInfo(key string, meta map[string]string) (artist, title, album string) {
	artist = meta["artist"]
	title = meta["title"]
	album = meta["album"]

	if album == "" {
		dir := filepath.Dir(key)
		if dir != "." && dir != "/" {
			album = filepath.Base(dir)
		}
	}

	if artist == "" || title == "" {
		base := strings.TrimSuffix(filepath.Base(key), filepath.Ext(key))
		if sep := " - "; strings.Contains(base, sep) {
			parts := strings.SplitN(base, sep, 2)
			if artist == "" {
				artist = strings.TrimSpace(parts[0])
			}
			if title == "" {
				title = strings.TrimSpace(parts[1])
			}
		} else if title == "" {
			title = strings.TrimSpace(base)
		}
	}

	return artist, title, album
}
