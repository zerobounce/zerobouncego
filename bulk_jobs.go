package zerobouncego

import (
	"fmt"
	"net/url"
	"strings"
)

// The S3 email finder / domain search jobs and the SFTP/FTP ones, documented at
//
//	https://www.zerobounce.net/docs/api-integrations/amazon-s3-bulk-api
//	https://www.zerobounce.net/docs/api-integrations/sftp_bulk_api
//
// They live on the bulkapi host WITHOUT the /v2 path, like the S3 jobs in
// priority3_bulk.go, and go out through postBulkRootForm.

// BulkFileJobOptions - the documented fields shared by the six S3 / SFTP file jobs.
//
// FileName is required by every job except ValidateSFTPFile, where the docs
// list it as optional: leaving it empty there uses the input bucket configured
// in your ZeroBounce settings. RemoveDuplicates and ReturnURL are omitted until
// set.
type BulkFileJobOptions struct {
	FileName         string
	HasHeaderRow     bool
	RemoveDuplicates *bool
	ReturnURL        string
}

// bulkRootFileJob - build and POST one of the six documented file jobs.
// Booleans go on the wire as the strings "true"/"false".
func bulkRootFileJob(path string, opts BulkFileJobOptions, fileNameRequired bool) (*FileValidationResponse, error) {
	if fileNameRequired && strings.TrimSpace(opts.FileName) == "" {
		return nil, fmt.Errorf("empty parameter: file_name")
	}
	fields := url.Values{}
	fields.Set("has_header_row", fmt.Sprintf("%t", opts.HasHeaderRow))
	if opts.FileName != "" {
		fields.Set("file_name", opts.FileName)
	}
	if opts.RemoveDuplicates != nil {
		fields.Set("remove_duplicates", fmt.Sprintf("%t", *opts.RemoveDuplicates))
	}
	if opts.ReturnURL != "" {
		fields.Set("return_url", opts.ReturnURL)
	}
	return postBulkRootForm(path, fields)
}

// EmailFinderS3File runs the email finder on a file in your Amazon S3 bucket
// (POST /emailFinderS3file). FileName is the full path, as returned by ListS3Files.
func EmailFinderS3File(opts BulkFileJobOptions) (*FileValidationResponse, error) {
	return bulkRootFileJob("/emailFinderS3file", opts, true)
}

// DomainSearchS3File runs domain search on a file in your Amazon S3 bucket
// (POST /domainSearchS3file).
func DomainSearchS3File(opts BulkFileJobOptions) (*FileValidationResponse, error) {
	return bulkRootFileJob("/domainSearchS3file", opts, true)
}

// SFTP / FTP bulk. The server, path and protocol are never sent by the SDK;
// they come from your ZeroBounce settings.

// ValidateSFTPFile validates a file from your SFTP/FTP server
// (POST /validateSFTPfile).
//
// This is the one job whose file_name the docs list as optional: leave
// opts.FileName empty to use the input bucket from your settings.
func ValidateSFTPFile(opts BulkFileJobOptions) (*FileValidationResponse, error) {
	return bulkRootFileJob("/validateSFTPfile", opts, false)
}

// ScoreSFTPFile scores a file from your SFTP/FTP server (POST /scoreSFTPfile).
// FileName is a plain name such as "my_file.txt".
func ScoreSFTPFile(opts BulkFileJobOptions) (*FileValidationResponse, error) {
	return bulkRootFileJob("/scoreSFTPfile", opts, true)
}

// EmailFinderSFTPFile runs the email finder on a file from your SFTP/FTP server
// (POST /emailFinderSFTPfile).
func EmailFinderSFTPFile(opts BulkFileJobOptions) (*FileValidationResponse, error) {
	return bulkRootFileJob("/emailFinderSFTPfile", opts, true)
}

// DomainSearchSFTPFile runs domain search on a file from your SFTP/FTP server
// (POST /domainSearchSFTPfile).
func DomainSearchSFTPFile(opts BulkFileJobOptions) (*FileValidationResponse, error) {
	return bulkRootFileJob("/domainSearchSFTPfile", opts, true)
}

// ExportSFTPFile exports a validated or scored file back to your SFTP/FTP
// server (POST /exportSFTPfile).
//
// Note the field name: this endpoint takes export_path, not the export_bucket
// that ExportS3File sends. Pass "" to use the export path from your settings.
func ExportSFTPFile(fileID, exportPath string) (*FileValidationResponse, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("empty parameter: file_id")
	}
	fields := url.Values{}
	fields.Set("file_id", fileID)
	if exportPath != "" {
		fields.Set("export_path", exportPath)
	}
	return postBulkRootForm("/exportSFTPfile", fields)
}
