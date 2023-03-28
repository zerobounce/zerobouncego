package zerobouncego

import "io"

// AiScoringSubmit - submit a file with emails for AI scoring
func AiScoringFileSubmit(csv_file CsvFile, remove_duplicate bool) (*FileValidationResponse, error) {
	return GenericFileSubmit(csv_file, remove_duplicate, ENDPOINT_FILE_SEND)
}

// BulkValidationFileStatus - check the percentage of completion of a file uploaded
// for AI scoring
func AiScoringFileStatus(file_id string) (*FileStatusResponse, error) {
	return GenericFileStatusCheck(file_id, ENDPOINT_FILE_STATUS)
}

// AiScoringResult - save a csv containing the results of the file previously sent,
// that corresponds to the given file ID parameter
func AiScoringResult(file_id string, file_writer io.WriteCloser) error {
	return GenericResultFetch(file_id, ENDPOINT_FILE_RESULT, file_writer)
}

// AiScoringFileDelete - cancel the validation process for a given file ID
func AiScoringFileDelete(file_id string) error {
	return GenericFileDelete(file_id, ENDPOINT_FILE_DELETE)
}
