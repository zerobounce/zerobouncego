package zerobouncego

import "net/url"

// DomainFormats part of the `FindEmailResponse` describing other domain formats
type DomainFormats struct {
	Format		string	`json:"format"`
	Confidence	string	`json:"confidence"`
}

// FindEmailResponse response structure for Find Email API
// `Confidence` field possible values: low, medium, high, unknown, undetermined
// (it is inconsistent as it can be either lowercase or uppercase)
type FindEmailResponse struct {
	Email				string			`json:"email"`
	Domain				string			`json:"domain"`
	Format				string			`json:"format"`
	Status				string			`json:"status"`
	SubStatus			string			`json:"sub_status"`
	Confidence			string			`json:"confidence"`
	DidYouMean			string			`json:"did_you_mean"`
	FailureReason		string			`json:"failure_reason"`
	OtherDomainFormats	[]DomainFormats	`json:"other_domain_formats"`
}

// FindEmail uses parameters to provide valid business email
func FindEmail(first_name, middle_name, last_name, domain string) (*FindEmailResponse, error) {
	var error_ error
	response := &FindEmailResponse{}

	request_parameters := url.Values{}
	request_parameters.Set("first_name", first_name)
	request_parameters.Set("middle_name", middle_name)
	request_parameters.Set("last_name", last_name)
	request_parameters.Set("domain", domain)
	url_to_request, error_ := PrepareURL(ENDPOINT_EMAIL_FINDER, request_parameters)
	if error_ != nil {
		return response, error_
	}

	error_ = DoGetRequest(url_to_request, response)
	return response, error_
}

// DomainSearch - attempts to detect possible patterns a specific company uses
func DomainSearch(domain string) (*FindEmailResponse, error) {
	return FindEmail("", "", "", domain)
}
