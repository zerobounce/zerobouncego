package zerobouncego

import "net/url"

// DomainFormats part of the `DomainSearchResponse` describing other domain formats
type DomainFormats struct {
	Format		string	`json:"format"`
	Confidence	string	`json:"confidence"`
}

// DomainSearchResponse response structure for DomainSearch API
// `Confidence` field possible values: low, medium, high, unknown, undetermined
// (it is inconsistent as it can be either lowercase or uppercase)
type DomainSearchResponse struct {
	Domain				string			`json:"domain"`
	CompanyName			string			`json:"company_name"`
	Format				string			`json:"format"`
	Confidence			string			`json:"confidence"`
	DidYouMean			string			`json:"did_you_mean"`
	FailureReason		string			`json:"failure_reason"`
	OtherDomainFormats	[]DomainFormats	`json:"other_domain_formats"`
}

func domainSearchInternal(domain, company_name string) (*DomainSearchResponse, error) {
	var error_ error
	response := &DomainSearchResponse{}

	request_parameters := url.Values{}
	if len(domain) > 0 {
		request_parameters.Set("domain", domain)
	}
	if len(company_name) > 0 {
		request_parameters.Set("company_name", company_name)
	}

	url_to_request, error_ := PrepareURL(ENDPOINT_EMAIL_FINDER, request_parameters)
	if error_ != nil {
		return response, error_
	}

	error_ = DoGetRequest(url_to_request, response)
	return response, error_
}

// DomainSearchByDomain attempts to detect possible patterns a specific company uses based on a given domain
func DomainSearchByDomain(domain string) (*DomainSearchResponse, error) {
	return domainSearchInternal(domain, "")
}

// DomainSearchByCompanyName attempts to detect possible patterns a specific company uses based on a given company name
func DomainSearchByCompanyName(company_name string) (*DomainSearchResponse, error) {
	return domainSearchInternal("", company_name)
}