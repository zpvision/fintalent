package accountingcompany

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminCompanyEndpointsRequireAdmin(t *testing.T) {
	handler := New(nil, nil, func(*http.Request) bool { return false })
	for _, test := range []struct {
		method string
		path   string
		handle http.HandlerFunc
	}{
		{http.MethodGet, "/api/admin/community/accounting-companies", handler.adminCompanies},
		{http.MethodPost, "/api/admin/community/accounting-companies/12/archive", handler.adminCompanyAction},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		recorder := httptest.NewRecorder()
		test.handle(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s %s: status=%d, want %d", test.method, test.path, recorder.Code, http.StatusForbidden)
		}
	}
}

func TestDecodeAcceptsServiceFieldsReturnedByAPI(t *testing.T) {
	request := httptest.NewRequest("PUT", "/api/accounting-companies/12", strings.NewReader(`{
		"name":"Новая бухгалтерская компания",
		"services":[{
			"id":30,
			"icon":"briefcase",
			"name":"Финансовый директор на аутсорсе",
			"price_from":1067,
			"price_type":"from_month",
			"service_id":12,
			"sort_order":0,
			"custom_name":""
		}]
	}`))
	recorder := httptest.NewRecorder()
	var input CompanyInput

	if !decode(recorder, request, &input) {
		t.Fatalf("decode rejected an API service object: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(input.Services) != 1 || input.Services[0].ServiceID == nil || *input.Services[0].ServiceID != 12 {
		t.Fatalf("unexpected decoded services: %#v", input.Services)
	}
}

func TestDecodeExplainsUnknownField(t *testing.T) {
	request := httptest.NewRequest("PUT", "/api/accounting-companies/12", strings.NewReader(`{"name":"Компания","unexpected":true}`))
	recorder := httptest.NewRecorder()
	var input CompanyInput

	if decode(recorder, request, &input) {
		t.Fatal("decode accepted an unknown field")
	}
	if !strings.Contains(recorder.Body.String(), "unexpected") {
		t.Fatalf("response does not identify the invalid field: %s", recorder.Body.String())
	}
}
