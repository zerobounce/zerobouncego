package zerobouncego

import "net/url"

// FindEmailResponse response structure for Find Email API
// `EmailConfidence` field possible values: low, medium, high
// (it is inconsistent as it can be either lowercase or uppercase)
type FindEmailResponse struct {
	Email				string			`json:"email"`
	EmailConfidence		string			`json:"email_confidence"`
	Domain				string			`json:"domain"`
	CompanyName			string			`json:"company_name"`
	DidYouMean			string			`json:"did_you_mean"`
	FailureReason		string			`json:"failure_reason"`
}

func findEmailInternal(domain, company_name, first_name, middle_name, last_name string) (*FindEmailResponse, error) {
	var error_ error
	response := &FindEmailResponse{}

	request_parameters := url.Values{}
	if len(domain) > 0 {
		request_parameters.Set("domain", domain)
	}
	if len(company_name) > 0 {
		request_parameters.Set("company_name", company_name)
	}
	if len(first_name) > 0 {
		request_parameters.Set("first_name", first_name)
	}
	if len(middle_name) > 0 {
		request_parameters.Set("middle_name", middle_name)
	}
	if len(last_name) > 0 {
		request_parameters.Set("last_name", last_name)
	}

	url_to_request, error_ := PrepareURL(ENDPOINT_EMAIL_FINDER, request_parameters)
	if error_ != nil {
		return response, error_
	}

	error_ = DoGetRequest(url_to_request, response)
	return response, error_
}

// FindEmailByDomain uses parameters to provide valid business email based on a given domain
func FindEmailByDomainFirstMiddleLastName(domain, first_name, middle_name, last_name string) (*FindEmailResponse, error) {
	return findEmailInternal(domain, "", first_name, middle_name, last_name)
}

// FindEmailByDomain uses parameters to provide valid business email based on a given domain
func FindEmailByDomainFirstLastName(domain, first_name, last_name string) (*FindEmailResponse, error) {
	return findEmailInternal(domain, "", first_name, "", last_name)
}

// FindEmailByDomain uses parameters to provide valid business email based on a given domain
func FindEmailByDomainFirstName(domain, first_name string) (*FindEmailResponse, error) {
	return findEmailInternal(domain, "", first_name, "", "")
}


// FindEmailByCompanyName uses parameters to provide valid business email based on a given company name
func FindEmailByCompanyFirstMiddleLastName(company_name, first_name, middle_name, last_name string) (*FindEmailResponse, error) {
	return findEmailInternal("", company_name, first_name, middle_name, last_name)
}

// FindEmailByCompanyName uses parameters to provide valid business email based on a given company name
func FindEmailByCompanyFirstLastName(company_name, first_name, last_name string) (*FindEmailResponse, error) {
	return findEmailInternal("", company_name, first_name, "", last_name)
}

// FindEmailByCompanyName uses parameters to provide valid business email based on a given company name
func FindEmailByCompanyFirstName(company_name, first_name string) (*FindEmailResponse, error) {
	return findEmailInternal("", company_name, first_name, "", "")
}


// FindEmail uses parameters to provide valid business email
//
// Deprecated: Use FindEmailBy... methods
func FindEmail(domain, first_name, middle_name, last_name string) (*FindEmailResponse, error) {
	return findEmailInternal(domain, "", first_name, middle_name, last_name)
}

// DomainSearch - attempts to detect possible patterns a specific company uses
//
// Deprecated: Use DomainSearchBy... methods
func DomainSearch(domain string) (*FindEmailResponse, error) {
	return findEmailInternal(domain, "", "", "", "")
}
