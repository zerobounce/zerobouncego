package zerobouncego

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// `null` in an optional response field reads as the field's empty value, never
// as an API error (standard.md §3). The calls whose answers carry these fields
// must accept one body holding every such null.
func TestNullOptionalFieldsReadAsEmpty(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")

	answersWithNullFields := map[string]bool{
		"Validate": true, "ValidateWithTimeout": true, "ValidateWithOptions": true,
		"ValidateBatch": true, "ValidateBatchWithOptions": true,
		"FindEmail": true, "FindEmailByDomainFirstMiddleLastName": true,
		"FindEmailByDomainFirstLastName": true, "FindEmailByDomainFirstName": true,
		"FindEmailByCompanyFirstMiddleLastName": true, "FindEmailByCompanyFirstLastName": true,
		"FindEmailByCompanyFirstName": true, "DomainSearch": true,
		"DomainSearchByDomain": true, "DomainSearchByCompanyName": true,
		"BulkValidationFileStatus": true, "BulkValidationSubmit": true,
	}
	body := `{"success":true,"file_id":null,"file_name":"f.csv","file_status":"Complete",` +
		`"complete_percentage":"100%","return_url":null,"upload_date":null,"error_reason":null,` +
		`"email_batch":null,"errors":null,"other_domain_formats":null,"processed_at":null,` +
		`"address":"a@b.c","status":"valid","sub_status":"","email":"a@b.c","domain":"b.c",` +
		`"format":"first","confidence":"high","did_you_mean":"","failure_reason":"","message":null}`

	covered := 0
	for _, call := range publicRequestCalls(client, warmup) {
		name := strings.TrimPrefix(call.name, "Client.")
		if !answersWithNullFields[name] {
			continue
		}
		covered++
		expectAccepted(t, call, fake, body)
	}
	if covered < len(answersWithNullFields) {
		t.Fatalf("only %d of %d calls in the sweep", covered, len(answersWithNullFields))
	}
}

// responseStructs are the typed answers the SDK decodes from a JSON body.
// TestEveryStructDecodedFromJSONIsSwept fails when a struct with json tags is
// added that is in neither this list nor the request or internal lists.
var responseStructs = []reflect.Type{
	reflect.TypeOf(ValidateResponse{}),
	reflect.TypeOf(CreditsResponse{}),
	reflect.TypeOf(ApiUsageResponse{}),
	reflect.TypeOf(ActivityDataResponse{}),
	reflect.TypeOf(ScoreEmailResponse{}),
	reflect.TypeOf(DomainFormats{}),
	reflect.TypeOf(DomainSearchResponse{}),
	reflect.TypeOf(FindEmailResponse{}),
	reflect.TypeOf(ValidateBatchResponse{}),
	reflect.TypeOf(EmailBatchError{}),
	reflect.TypeOf(FileValidationResponse{}),
	reflect.TypeOf(FileStatusResponse{}),
	reflect.TypeOf(S3ListFileEntry{}),
}

// Structs with json tags that are sent, not decoded, or are internal checkers.
var notDecodedStructs = map[string]bool{
	"CsvFile": true, "EmailToValidate": true, "ListEvaluatorCsvFile": true,
	"validateRequiredFields": true,
}

// requiredKeys are the fields a call cannot answer without: null there is an
// error, checked on the calls below rather than by the decoder.
var requiredKeys = map[string][]string{
	"ValidateResponse": {"address", "status"},
	"CreditsResponse":  {"Credits"},
}

func isRequiredKey(typ reflect.Type, key string) bool {
	for _, required := range requiredKeys[typ.Name()] {
		if required == key {
			return true
		}
	}
	return false
}

// jsonKeys are the keys a struct decodes: its json tag name, or the field name.
func jsonKeys(typ reflect.Type) []string {
	var keys []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := field.Name
		if tag, ok := field.Tag.Lookup("json"); ok {
			if tag == "-" {
				continue
			}
			if tagName := strings.Split(tag, ",")[0]; tagName != "" {
				name = tagName
			}
		}
		keys = append(keys, name)
	}
	return keys
}

// Every field of every typed answer reads `null` as its zero value, so the
// decoder accepts it, except the fields a call cannot answer without. The keys
// come from the struct declarations, not from a list written here.
func TestEveryResponseFieldReadsNullAsEmpty(t *testing.T) {
	for _, typ := range responseStructs {
		keys := jsonKeys(typ)
		if len(keys) == 0 {
			t.Fatalf("%s has no decodable fields", typ.Name())
		}
		nulls := make([]string, 0, len(keys))
		for _, key := range keys {
			if isRequiredKey(typ, key) {
				continue
			}
			nulls = append(nulls, `"`+key+`":null`)
			if err := json.Unmarshal([]byte(`{"`+key+`":null}`), reflect.New(typ).Interface()); err != nil {
				t.Errorf("%s.%s: null is an error: %v", typ.Name(), key, err)
			}
		}
		if err := json.Unmarshal([]byte("{"+strings.Join(nulls, ",")+"}"), reflect.New(typ).Interface()); err != nil {
			t.Errorf("%s with every field null: %v", typ.Name(), err)
		}
	}
}

// The required fields: null is an error on the calls that carry them.
func TestNullRequiredFieldsAreErrorsOnTheirCalls(t *testing.T) {
	useFake(t)
	fake := &fixedBodyTransport{}
	http.DefaultTransport = fake
	client, warmup := pointEverythingAt(t, "https://api.example.test")
	calls := map[string]requestCall{}
	for _, call := range publicRequestCalls(client, warmup) {
		calls[strings.TrimPrefix(call.name, "Client.")] = call
	}
	cases := []struct{ call, body string }{
		{"Validate", `{"address":null,"status":"valid"}`},
		{"Validate", `{"address":"a@b.c","status":null}`},
		{"ValidateBatch", `{"email_batch":[{"address":null,"status":"valid"}],"errors":[]}`},
		{"ValidateBatch", `{"email_batch":[{"address":"a@b.c","status":null}],"errors":[]}`},
		{"GetCredits", `{"Credits":null}`},
	}
	for _, c := range cases {
		call, ok := calls[c.call]
		if !ok {
			t.Fatalf("no %s call in the sweep", c.call)
		}
		expectStatus200Error(t, call, fake, c.body)
	}
}

// Every struct in the package with json tags is either swept as a response or
// listed as not decoded, so a new response type cannot skip the null sweep.
func TestEveryStructDecodedFromJSONIsSwept(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	swept := map[string]bool{}
	for _, typ := range responseStructs {
		swept[typ.Name()] = true
	}
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(node ast.Node) bool {
				spec, ok := node.(*ast.TypeSpec)
				if !ok {
					return true
				}
				structType, ok := spec.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range structType.Fields.List {
					if field.Tag != nil && strings.Contains(field.Tag.Value, `json:"`) {
						if !swept[spec.Name.Name] && !notDecodedStructs[spec.Name.Name] {
							t.Errorf("%s has json tags but is neither swept nor listed as not decoded", spec.Name.Name)
						}
						break
					}
				}
				return true
			})
		}
	}
	for name := range notDecodedStructs {
		if !declaredInPackage(packages, name) {
			t.Errorf("%s is listed as not decoded but is not declared", name)
		}
	}
}

func declaredInPackage(packages map[string]*ast.Package, name string) bool {
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range gen.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == name {
						return true
					}
				}
			}
		}
	}
	return false
}
