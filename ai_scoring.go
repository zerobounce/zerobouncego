package zerobouncego

import "io"

// AiScoringFileSubmit - submit a file with emails for AI scoring. It always
// sends remove_duplicate as given.
//
// Deprecated: use AiScoringFileSubmitWithOptions, which leaves
// remove_duplicate to the API's default (true) unless you set it.
func AiScoringFileSubmit(csv_file CsvFile, remove_duplicate bool) (*FileValidationResponse, error) {
	return GenericFileSubmit(csv_file, remove_duplicate, ENDPOINT_SCORING_SEND)
}

// AiScoringFileSubmitWithOptions - submit a file with emails for AI scoring.
// remove_duplicate is sent only when opts sets it.
func AiScoringFileSubmitWithOptions(csv_file CsvFile, opts *SendFileOptions) (*FileValidationResponse, error) {
	return GenericFileSubmitWithOptions(csv_file, ENDPOINT_SCORING_SEND, opts)
}

// BulkValidationFileStatus - check the percentage of completion of a file uploaded
// for AI scoring
func AiScoringFileStatus(file_id string) (*FileStatusResponse, error) {
	return GenericFileStatusCheck(file_id, ENDPOINT_SCORING_STATUS)
}

// AiScoringResult - save a csv containing the results of the file previously sent,
// that corresponds to the given file ID parameter
func AiScoringResult(file_id string, file_writer io.Writer) error {
	return GenericResultFetch(file_id, ENDPOINT_SCORING_RESULT, file_writer)
}

// AiScoringResultWithOptions - AI scoring getfile with optional download_type (activity_data is not sent).
func AiScoringResultWithOptions(file_id string, file_writer io.Writer, opts *GetFileOptions) error {
	return genericResultFetch(file_id, ENDPOINT_SCORING_RESULT, file_writer, opts, true)
}

// AiScoringFileDelete - delete the result file associated with a file ID
func AiScoringFileDelete(file_id string) (*FileValidationResponse, error) {
	return GenericFileDelete(file_id, ENDPOINT_SCORING_DELETE)
}
