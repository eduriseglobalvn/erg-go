package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// GDriveClient handles interaction with Google Drive API v3.
type GDriveClient struct {
	service *drive.Service
	folder  string // Root folder ID for uploads
}

// NewGDriveClient creates a new Google Drive client using a service account JSON.
// credentialJSON can be the raw JSON content or a path to the file.
func NewGDriveClient(ctx context.Context, credentialJSON string, rootFolderID string) (*GDriveClient, error) {
	var opts []option.ClientOption
	if credentialJSON != "" {
		if credentialJSON[0] == '{' {
			opts = append(opts, option.WithCredentialsJSON([]byte(credentialJSON)))
		} else {
			opts = append(opts, option.WithCredentialsFile(credentialJSON))
		}
	}

	srv, err := drive.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gdrive.new_service: %w", err)
	}

	return &GDriveClient{
		service: srv,
		folder:  rootFolderID,
	}, nil
}

// Upload uploads a file to a specific folder on Google Drive.
// It returns the Google Drive File ID.
func (c *GDriveClient) Upload(ctx context.Context, data io.Reader, filename string, mimeType string, parentFolderID string) (string, error) {
	if parentFolderID == "" {
		parentFolderID = c.folder
	}

	f := &drive.File{
		Name:     filename,
		Parents:  []string{parentFolderID},
		MimeType: mimeType,
	}

	res, err := c.service.Files.Create(f).Media(data).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gdrive.upload: %w", err)
	}

	return res.Id, nil
}

// Download returns a reader for the file content.
func (c *GDriveClient) Download(ctx context.Context, fileID string) (io.ReadCloser, error) {
	res, err := c.service.Files.Get(fileID).Download()
	if err != nil {
		return nil, fmt.Errorf("gdrive.download: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("gdrive.download: unexpected status %d", res.StatusCode)
	}

	return res.Body, nil
}

// Delete removes a file from Google Drive.
func (c *GDriveClient) Delete(ctx context.Context, fileID string) error {
	err := c.service.Files.Delete(fileID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("gdrive.delete: %w", err)
	}
	return nil
}

// GetWebLink returns the webViewLink for the file.
func (c *GDriveClient) GetWebLink(ctx context.Context, fileID string) (string, error) {
	f, err := c.service.Files.Get(fileID).Fields("webViewLink").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gdrive.get_link: %w", err)
	}
	return f.WebViewLink, nil
}

// CreateFolder creates a new folder in Google Drive and returns its ID.
func (c *GDriveClient) CreateFolder(ctx context.Context, name string, parentFolderID string) (string, error) {
	if parentFolderID == "" {
		parentFolderID = c.folder
	}

	f := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentFolderID},
	}

	res, err := c.service.Files.Create(f).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gdrive.create_folder: %w", err)
	}

	return res.Id, nil
}
