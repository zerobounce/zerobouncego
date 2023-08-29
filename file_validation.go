package zerobouncego

import "io"

// BulkValidationSubmit - submit a file with emails for validation
// Required columns: EmailAddressColumn
// Optional columns: FirstNameColumn, LastNameColumn, GenderColumn, IpAddressColumn
func BulkValidationSubmit(csv_file CsvFile, remove_duplicate bool) (*FileValidationResponse, error) {
	if csv_file.ColumnsNotSet() {
		// if no column is set, fallback the required column to index 1
		csv_file.EmailAddressColumn = 1
	}
	return GenericFileSubmit(csv_file, remove_duplicate, ENDPOINT_FILE_SEND)
}

// BulkValidationFileStatus - check the percentage of completion of a file uploaded
// for bulk validation
func BulkValidationFileStatus(file_id string) (*FileStatusResponse, error) {
	return GenericFileStatusCheck(file_id, ENDPOINT_FILE_STATUS)
}

// BulkValidationResult - save a csv containing the results of the file with the given file ID
func BulkValidationResult(file_id string, file_writer io.Writer) error {
	return GenericResultFetch(file_id, ENDPOINT_FILE_RESULT, file_writer)
}

// BulkValidationFileDelete - delete the result file associated with a file ID
func BulkValidationFileDelete(file_id string) (*FileValidationResponse, error) {
	return GenericFileDelete(file_id, ENDPOINT_FILE_DELETE)
}
