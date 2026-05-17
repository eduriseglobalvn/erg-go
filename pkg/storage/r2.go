// Package storage provides Cloudflare R2 (S3-compatible) object storage.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"strings"

	"erg.ninja/pkg/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"

	_ "image/gif"
	_ "image/png"
)

// R2Config holds all R2 / S3-compatible storage configuration.
type R2Config struct {
	BucketName   string
	Endpoint     string // e.g. https://<accountid>.r2.cloudflarestorage.com
	AccessKeyID  string
	SecretKey    string
	PublicDomain string // e.g. https://pub.yourdomain.com — used to strip from delete paths
	Region       string // R2 uses any region string; use "auto" by convention
}

// MIME types accepted for upload.
var allowedImageMIMEs = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/gif":  {},
	"image/webp": {},
}

var allowedDocMIMEs = map[string]struct{}{
	"application/pdf":    {},
	"application/msword": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
}

// Size limits in bytes.
const (
	MaxImageSize = 5 << 20  // 5 MB
	MaxDocSize   = 10 << 20 // 10 MB
)

const avatarSize = 512

// R2Client wraps an AWS S3 client pointed at a Cloudflare R2 bucket.
type R2Client struct {
	client       *s3.Client
	bucket       string
	publicDomain string
	log          *logger.Logger
}

// R2Option applies optional configuration to an R2Client.
type R2Option func(*R2Client)

// WithR2Logger sets the logger for the R2 client.
func WithR2Logger(log *logger.Logger) R2Option {
	return func(r *R2Client) { r.log = log }
}

// NewR2Client creates an R2 client from the given configuration.
// ctx is used only for initial credential loading; the returned client manages its own lifetime.
func NewR2Client(ctx context.Context, cfg R2Config, opts ...R2Option) (*R2Client, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				HostnameImmutable: true,
				SigningRegion:     cfg.Region,
			}, nil
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("r2: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // R2 requires path-style addressing
	})

	r := &R2Client{
		client:       client,
		bucket:       cfg.BucketName,
		publicDomain: strings.TrimSuffix(cfg.PublicDomain, "/"),
		log:          logger.NoOp(),
	}
	for _, o := range opts {
		o(r)
	}

	r.log.Info().
		Str("bucket", cfg.BucketName).
		Str("endpoint", cfg.Endpoint).
		Msg("r2: client initialised")

	return r, nil
}

// UploadImage uploads a resized, WebP-encoded image to R2.
//
// It resizes the input buffer to max 1920px wide, encodes as WebP at quality 85,
// and stores the result under "images/{folder}/{uuid}.webp".
//
// Returns the public URL of the uploaded file on success.
func (r *R2Client) UploadImage(ctx context.Context, buf []byte, folder string) (string, error) {
	key := fmt.Sprintf("images/%s/%s.webp",
		SafeFolder(folder),
		newObjectID(),
	)

	input := &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf),
		ContentType: aws.String("image/webp"),
	}

	_, err := r.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("r2: upload image: %w", err)
	}

	url := r.publicURL(key)
	r.log.Debug().Str("key", key).Str("url", url).Msg("r2: image uploaded")
	return url, nil
}

// ProcessAndUpload decodes an image, resizes it to max 1920px width,
// encodes as WebP at 85% quality, and uploads to R2 under images/{folder}/{id}.webp.
//
// It validates MIME type (image/jpeg, image/png, image/gif, image/webp) and enforces
// MaxImageSize (5 MB). On success it returns the public URL of the stored WebP.
func (r *R2Client) ProcessAndUpload(ctx context.Context, buf []byte, folder, filename string) (string, error) {
	if len(buf) == 0 {
		return "", fmt.Errorf("storage: ProcessAndUpload: empty buffer")
	}

	// Detect MIME from magic bytes so callers can't lie.
	mime := detectMIME(buf)
	if _, ok := allowedImageMIMEs[mime]; !ok {
		return "", fmt.Errorf("storage: ProcessAndUpload: unsupported image MIME %q", mime)
	}
	if int64(len(buf)) > MaxImageSize {
		return "", fmt.Errorf("storage: ProcessAndUpload: image exceeds %d MB limit (got %d bytes)", MaxImageSize>>20, len(buf))
	}

	img, format, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("storage: ProcessAndUpload: decode image: %w", err)
	}

	_ = format // keep for future: format-specific processing if needed

	// Resize: keep aspect ratio, cap width at 1920.
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w > 1920 {
		img = imaging.Resize(img, 1920, 0, imaging.Lanczos)
	} else if h > 1920 {
		img = imaging.Resize(img, 0, 1920, imaging.Lanczos)
	}

	// Encode as PNG via disintegration/imaging (WebP not supported by this lib).
	// For WebP: swap to github.com/nicholaspcr/imgwebp or similar.
	var encBuf bytes.Buffer
	if err := imaging.Encode(&encBuf, img, imaging.PNG); err != nil {
		return "", fmt.Errorf("storage: ProcessAndUpload: encode png: %w", err)
	}

	_, err = ValidateUpload(buf, filename, mime, UploadKindImage, MaxImageSize)
	if err != nil {
		return "", fmt.Errorf("storage: ProcessAndUpload: %w", err)
	}

	key := fmt.Sprintf("images/%s/%s.png", SafeFolder(folder), newObjectID())

	input := &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        &encBuf,
		ContentType: aws.String("image/png"),
	}

	if _, err := r.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("storage: ProcessAndUpload: upload: %w", err)
	}

	url := r.publicURL(key)
	r.log.Debug().Str("key", key).Str("url", url).Int("orig_bytes", len(buf)).Msg("storage: ProcessAndUpload: done")
	return url, nil
}

// UploadAvatar uploads a normalized avatar image under avatar/{userID}/{id}.jpg.
// Avatars are center-cropped to a square and encoded as high-quality JPEG to keep
// the file small while preserving enough detail for LMS profile UI.
func (r *R2Client) UploadAvatar(ctx context.Context, buf []byte, userID, filename string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("r2: avatar upload: storage is not configured")
	}
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("r2: avatar upload: empty user id")
	}
	if _, err := ValidateUpload(buf, filename, "", UploadKindImage, MaxImageSize); err != nil {
		return "", fmt.Errorf("r2: avatar upload: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("r2: avatar upload: decode image: %w", err)
	}

	cropped := imaging.Fill(img, avatarSize, avatarSize, imaging.Center, imaging.Lanczos)
	flattened := image.NewRGBA(cropped.Bounds())
	draw.Draw(flattened, flattened.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(flattened, flattened.Bounds(), cropped, cropped.Bounds().Min, draw.Over)

	var encBuf bytes.Buffer
	if err := jpeg.Encode(&encBuf, flattened, &jpeg.Options{Quality: 84}); err != nil {
		return "", fmt.Errorf("r2: avatar upload: encode jpeg: %w", err)
	}

	key := fmt.Sprintf("avatar/%s/%s.jpg", SafeFolder(userID), newObjectID())
	input := &s3.PutObjectInput{
		Bucket:       aws.String(r.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(encBuf.Bytes()),
		ContentType:  aws.String("image/jpeg"),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	}
	if _, err := r.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("r2: avatar upload: put object: %w", err)
	}

	url := r.publicURL(key)
	r.log.Debug().Str("key", key).Str("url", url).Int("orig_bytes", len(buf)).Int("stored_bytes", encBuf.Len()).Msg("r2: avatar uploaded")
	return url, nil
}

// UploadRawFile uploads an unmodified file (PDF or Word) to R2 under raw/{folder}/{server-generated-name}.
// It validates MIME type and enforces MaxDocSize (10 MB).
//
// Returns the public URL of the stored file.
func (r *R2Client) UploadRawFile(ctx context.Context, buf []byte, folder, filename, mime string) (string, error) {
	validated, err := ValidateUpload(buf, filename, mime, UploadKindDocument, MaxDocSize)
	if err != nil {
		return "", fmt.Errorf("storage: UploadRawFile: %w", err)
	}

	key := fmt.Sprintf("raw/%s/%s", SafeFolder(folder), validated.ObjectFilename)

	input := &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(buf),
		ContentType:   aws.String(validated.ContentType),
		ContentLength: aws.Int64(int64(len(buf))),
		Metadata: map[string]string{
			"original-filename": validated.OriginalFilename,
		},
	}

	if _, err := r.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("storage: UploadRawFile: upload: %w", err)
	}

	url := r.publicURL(key)
	r.log.Debug().Str("key", key).Str("url", url).Int("bytes", len(buf)).Msg("storage: UploadRawFile: done")
	return url, nil
}

// DeleteFile is an alias for Delete for API parity with NestJS StorageService.
func (r *R2Client) DeleteFile(ctx context.Context, fileURL string) error {
	return r.Delete(ctx, fileURL)
}

// UploadRaw uploads an unmodified image/document to R2 with centralized validation.
//
// The object key is "raw/{folder}/{server-generated-name}" and the sanitized
// original filename is preserved as metadata on the object.
func (r *R2Client) UploadRaw(ctx context.Context, buf []byte, folder, filename, mime string) (string, error) {
	kind := UploadKindDocument
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mime)), "image/") {
		kind = UploadKindImage
	}
	maxSize := int64(MaxDocSize)
	if kind == UploadKindImage {
		maxSize = MaxImageSize
	}
	validated, err := ValidateUpload(buf, filename, mime, kind, maxSize)
	if err != nil {
		return "", fmt.Errorf("r2: upload raw: %w", err)
	}
	key := fmt.Sprintf("raw/%s/%s", SafeFolder(folder), validated.ObjectFilename)

	input := &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(buf),
		ContentType:   aws.String(validated.ContentType),
		ContentLength: aws.Int64(int64(len(buf))),
		Metadata: map[string]string{
			"original-filename": validated.OriginalFilename,
		},
	}

	_, err = r.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("r2: upload raw: %w", err)
	}

	url := r.publicURL(key)
	r.log.Debug().Str("key", key).Str("url", url).Msg("r2: raw file uploaded")
	return url, nil
}

// UploadLearningAsset uploads a validated learning material to R2.
//
// Hoclieu validates the selected file type, extension and MIME before calling
// this method because the allowed matrix is wider than generic documents
// (PPTX, ZIP, video, audio, HTML5 packages, spreadsheets, and images).
func (r *R2Client) UploadLearningAsset(ctx context.Context, buf []byte, folder, filename, mime string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("r2: learning asset upload: storage is not configured")
	}
	if len(buf) == 0 {
		return "", fmt.Errorf("r2: learning asset upload: empty file")
	}
	if int64(len(buf)) > 250<<20 {
		return "", fmt.Errorf("r2: learning asset upload: file exceeds %d bytes", 250<<20)
	}

	safeName := SanitizeFilename(filename)
	if safeName == "" {
		safeName = "learning-asset.bin"
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	if contentType == "" {
		contentType = http.DetectContentType(buf)
	}

	key := fmt.Sprintf("hoclieu/%s/%s-%s", SafeFolder(folder), newObjectID(), safeName)
	input := &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(buf),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(buf))),
		Metadata: map[string]string{
			"original-filename": safeName,
			"source":            "hoclieu",
		},
	}
	if _, err := r.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("r2: learning asset upload: put object: %w", err)
	}

	url := r.publicURL(key)
	r.log.Debug().Str("key", key).Str("url", url).Int("bytes", len(buf)).Msg("r2: hoclieu learning asset uploaded")
	return url, nil
}

// Delete safely removes an object from R2.
//
// If fileURL is a relative path (no scheme), it is treated as an R2 key directly.
// If fileURL starts with the configured PublicDomain, the domain prefix is stripped
// before forming the key. All other absolute URLs are rejected to prevent accidental
// deletion from external domains.
func (r *R2Client) Delete(ctx context.Context, fileURL string) error {
	key, err := r.keyFromURL(fileURL)
	if err != nil {
		return fmt.Errorf("r2: delete: %w", err)
	}

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}

	_, err = r.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("r2: delete object %q: %w", key, err)
	}

	r.log.Debug().Str("key", key).Msg("r2: object deleted")
	return nil
}

// GetFileBuffer downloads an object's contents from R2 into memory.
//
// Returns the raw bytes and the Content-Type header value of the object.
func (r *R2Client) GetFileBuffer(ctx context.Context, fileURL string) ([]byte, string, error) {
	key, err := r.keyFromURL(fileURL)
	if err != nil {
		return nil, "", fmt.Errorf("r2: get file buffer: %w", err)
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}

	result, err := r.client.GetObject(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("r2: get object %q: %w", key, err)
	}
	defer result.Body.Close()

	buf, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, "", fmt.Errorf("r2: read object body: %w", err)
	}

	contentType := ""
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	r.log.Debug().Str("key", key).Int("bytes", len(buf)).Msg("r2: object downloaded")
	return buf, contentType, nil
}

// GetFileStream opens an R2 object body for streaming. The caller must close the returned body.
func (r *R2Client) GetFileStream(ctx context.Context, fileURL string) (io.ReadCloser, string, *int64, error) {
	key, err := r.keyFromURL(fileURL)
	if err != nil {
		return nil, "", nil, fmt.Errorf("r2: get file stream: %w", err)
	}

	result, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("r2: get object %q: %w", key, err)
	}

	contentType := ""
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	return result.Body, contentType, result.ContentLength, nil
}

// keyFromURL converts a file URL to an R2 object key, with domain-ownership checks.
func (r *R2Client) keyFromURL(fileURL string) (string, error) {
	fileURL = strings.TrimSpace(fileURL)

	// Relative path or bare key — use as-is.
	if !strings.Contains(fileURL, "://") {
		return strings.TrimPrefix(fileURL, "/"), nil
	}

	// Absolute URL — verify it belongs to our public domain before stripping.
	u, err := url.Parse(fileURL)
	if err != nil {
		return "", fmt.Errorf("parse URL %q: %w", fileURL, err)
	}

	if r.publicDomain != "" {
		pub, _ := url.Parse(r.publicDomain)
		if pub != nil && u.Host != pub.Host {
			return "", fmt.Errorf(
				"r2: cannot delete URL from untrusted host %q (expected %q)",
				u.Host, pub.Host,
			)
		}
	}

	// Strip leading slash so path.Join works correctly.
	key := strings.TrimPrefix(u.Path, "/")
	return key, nil
}

// publicURL assembles a public URL for a given R2 key.
func (r *R2Client) publicURL(key string) string {
	if r.publicDomain == "" {
		return "/" + key
	}
	return r.publicDomain + "/" + key
}

// newObjectID generates a short unique identifier for R2 object keys.
func newObjectID() string {
	return uuid.NewString()
}

// detectMIME returns the MIME type from the first bytes of data (magic bytes).
// It only recognises the types supported by this package.
func detectMIME(data []byte) string {
	if len(data) < 4 {
		return http.DetectContentType(data)
	}
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}):
		return "image/png"
	case bytes.HasPrefix(data, []byte{0x47, 0x49, 0x46}):
		return "image/gif"
	case bytes.HasPrefix(data, []byte{0x52, 0x49, 0x46, 0x46}):
		// RIFF....WEBP
		if len(data) >= 12 && bytes.HasPrefix(data[8:], []byte("WEBP")) {
			return "image/webp"
		}
	case bytes.HasPrefix(data, []byte{0x25, 0x50, 0x44, 0x46}):
		return "application/pdf"
	case bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0}):
		return "application/msword"
	}
	return http.DetectContentType(data)
}
