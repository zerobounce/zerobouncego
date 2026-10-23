package zerobouncego

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
)

// bulkAPIRoot derives https://bulkapi.zerobounce.net from BULK_URI (strips trailing /v2).
func bulkAPIRoot() string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(BULK_URI), "/")
	trimmed = strings.TrimSuffix(trimmed, "/v2")
	return strings.TrimSuffix(trimmed, "/")
}

func bulkV2Base() string {
	return strings.TrimSuffix(strings.TrimSpace(BULK_URI), "/")
}

func emailFinderBaseURL() string {
	return bulkAPIRoot() + "/email-finder"
}

func domainSearchBaseURL() string {
	return bulkAPIRoot() + "/domain-search"
}

func appendMultipartFields(writer *multipart.Writer, fields map[string]string) error {
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	return nil
}

func postBulkRootMultipart(requestURL string, file io.Reader, fileName string, fields map[string]string) (*FileValidationResponse, error) {
	if err := packageConfig().requireKey(); err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("empty parameter: file")
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	if err := appendMultipartFields(writer, fields); err != nil {
		return nil, err
	}
	if err := writer.WriteField("api_key", API_KEY); err != nil {
		return nil, err
	}

	fileWriter, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if _, err = fileWriter.Write(contents); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}

	responseHTTP, err := timedHTTPClient().Post(requestURL, writer.FormDataContentType(), &buffer)
	if err != nil {
		return nil, err
	}
	defer responseHTTP.Body.Close()
	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return nil, err
	}
	responseObject := &FileValidationResponse{}
	if err = decodeJSONBody(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), body, responseObject); err != nil {
		return nil, err
	}
	return responseObject, nil
}

func postBulkRootForm(path string, fields url.Values) (*FileValidationResponse, error) {
	if err := packageConfig().requireKey(); err != nil {
		return nil, err
	}
	if fields == nil {
		fields = url.Values{}
	}
	fields.Set("api_key", API_KEY)

	requestURL, err := url.JoinPath(bulkAPIRoot(), strings.TrimPrefix(path, "/"))
	if err != nil {
		return nil, err
	}

	responseHTTP, err := timedHTTPClient().PostForm(requestURL, fields)
	if err != nil {
		return nil, err
	}
	defer responseHTTP.Body.Close()
	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return nil, err
	}
	responseObject := &FileValidationResponse{}
	if err = decodeJSONBody(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), body, responseObject); err != nil {
		return nil, err
	}
	return responseObject, nil
}

func bulkRootFileStatus(baseURL, fileID string) (*FileStatusResponse, error) {
	if err := packageConfig().requireKey(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("empty parameter: file_id")
	}
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", fileID)

	requestURL, err := url.JoinPath(baseURL, "filestatus")
	if err != nil {
		return nil, err
	}
	requestURL = fmt.Sprintf("%s?%s", requestURL, params.Encode())

	responseHTTP, err := httpGet(requestURL)
	if err != nil {
		return nil, err
	}
	defer responseHTTP.Body.Close()
	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return nil, err
	}
	responseObject := &FileStatusResponse{}
	if err = decodeJSONBody(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), body, responseObject); err != nil {
		return nil, err
	}
	return responseObject, nil
}

func bulkRootResultFetch(baseURL, fileID string, fileWriter io.Writer) error {
	if err := packageConfig().requireKey(); err != nil {
		return err
	}
	if strings.TrimSpace(fileID) == "" {
		return fmt.Errorf("empty parameter: file_id")
	}
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", fileID)

	requestURL, err := url.JoinPath(baseURL, "getfile")
	if err != nil {
		return err
	}
	requestURL = fmt.Sprintf("%s?%s", requestURL, params.Encode())

	responseHTTP, err := httpGet(requestURL)
	if err != nil {
		return err
	}
	defer responseHTTP.Body.Close()

	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return err
	}

	bodyStr := string(body)
	if err = getFileBodyError(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), bodyStr); err != nil {
		return err
	}

	_, err = fileWriter.Write(body)
	return err
}

func bulkRootFileDelete(baseURL, fileID string) (*FileValidationResponse, error) {
	if err := packageConfig().requireKey(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("empty parameter: file_id")
	}
	params := url.Values{}
	params.Set("api_key", API_KEY)
	params.Set("file_id", fileID)

	requestURL, err := url.JoinPath(baseURL, "deletefile")
	if err != nil {
		return nil, err
	}
	requestURL = fmt.Sprintf("%s?%s", requestURL, params.Encode())

	responseHTTP, err := httpGet(requestURL)
	if err != nil {
		return nil, err
	}
	defer responseHTTP.Body.Close()
	body, err := io.ReadAll(responseHTTP.Body)
	if err != nil {
		return nil, err
	}
	responseObject := &FileValidationResponse{}
	if err = decodeJSONBody(responseHTTP.StatusCode, responseHTTP.Header.Get("Content-Type"), body, responseObject); err != nil {
		return nil, err
	}
	responseObject.FileId = fileID
	return responseObject, nil
}

func formatBoolField(value bool) string {
	return fmt.Sprintf("%v", value)
}

func optionalIntField(column int) (string, bool) {
	if column == 0 {
		return "", false
	}
	return fmt.Sprintf("%d", column), true
}
