package storage

import (
	"context"
	"io"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"cloud.google.com/go/storage"
)

type GCSStorage struct {
	ctx    context.Context
	client *storage.Client
	bucket *storage.BucketHandle
	prefix string
}

func NewGCSBackend(
	ctx context.Context,
	bucketName string,
	prefix string,
	endpoint string,
) (Storage, error) {
	log.WithFields(log.Fields{
		"bucket":   bucketName,
		"prefix":   prefix,
		"endpoint": endpoint,
	}).Debug("gcs: making backend")

	var options []option.ClientOption
	if endpoint != "" {
		options = append(options, option.WithEndpoint(endpoint))
	}

	client, err := storage.NewClient(ctx, options...)
	if err != nil {
		return nil, err
	}

	// Check the bucket exists
	bucket := client.Bucket(bucketName)
	if _, err := bucket.Attrs(ctx); err != nil {
		return nil, err
	}

	backend := GCSStorage{
		ctx:    ctx,
		client: client,
		bucket: bucket,
		prefix: prefix,
	}
	return &backend, nil
}

func (b *GCSStorage) Exists(pth string) (bool, error) {
	log.WithField("path", path.Join(b.prefix, pth)).Trace("gcs: check exists")
	_, err := b.Size(pth)
	return err == nil, err
}

func (b *GCSStorage) Size(pth string) (int64, error) {
	pth = path.Join(b.prefix, pth)
	log.WithField("path", pth).Trace("gcs: get size")
	attrs, err := b.bucket.Object(pth).Attrs(context.Background())
	if err == storage.ErrObjectNotExist {
		err = os.ErrNotExist
	}
	if err != nil {
		return 0, err
	}
	return attrs.Size, nil
}

func (b *GCSStorage) GetFile(pth string) (io.ReadCloser, error) {
	pth = path.Join(b.prefix, pth)
	log.WithField("path", pth).Trace("gcs: get file")
	r, err := b.bucket.Object(pth).NewReader(context.Background())
	if err == storage.ErrObjectNotExist {
		// TODO: Check this is right
		//lint:ignore SA4006 Ignore unused function temporarily
		err = os.ErrNotExist
	}
	return r, nil
}

func (b *GCSStorage) PutFile(pth string, in io.ReadCloser) error {
	pth = path.Join(b.prefix, pth)
	log.WithField("path", pth).Trace("gcs: get file")
	w := b.bucket.Object(pth).NewWriter(context.Background())
	if _, err := io.Copy(w, in); err != nil {
		return err
	}
	in.Close()
	return w.Close()
}

func (b *GCSStorage) ListFiles(pth string) (chan string, chan error) {
	prefix := path.Join(b.prefix, pth)
	ch := make(chan string)
	errs := make(chan error)

	go func() {
		log.WithField("path", pth).Trace("gcs: list files")
		defer close(ch)
		defer close(errs)

		iter := b.bucket.Objects(context.Background(), &storage.Query{Prefix: prefix})
		for {
			object, err := iter.Next()
			if err == iterator.Done {
				return
			} else if err != nil {
				errs <- err
			} else {
				// TODO: Check Name is right
				ch <- object.Name
			}
		}
	}()

	return ch, errs
}

func (b *GCSStorage) CanListFiles() bool {
	log.Trace("gcs: can list files")
	return true
}

func (b *GCSStorage) Close() error {
	log.Trace("gcs: close")
	return b.client.Close()
}
