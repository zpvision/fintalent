package academy

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCourseCommissionBasisPoints(t *testing.T) {
	for _, v := range []struct {
		standard, partner int
		referral          bool
		want              int
	}{{2000, 1200, false, 2000}, {2000, 1200, true, 1200}, {1750, 975, true, 975}, {0, 0, false, 0}} {
		if got := commissionBasisPoints(v.standard, v.partner, v.referral); got != v.want {
			t.Fatalf("commission %d != %d", got, v.want)
		}
	}
}

func TestManualPaymentEarlyValidation(t *testing.T) {
	h := &Handler{}
	cases := []struct {
		method, body string
		path         []string
		cross        bool
		want         int
	}{{"GET", `{}`, []string{"1", "paid"}, false, 405}, {"POST", `{}`, []string{"1", "paid"}, true, 403}, {"POST", `{}`, []string{"2", "other"}, false, 404}, {"POST", `{}`, []string{"bad", "paid"}, false, 400}, {"POST", `{"confirm":false,"version_id":1}`, []string{"1", "paid"}, false, 400}, {"POST", `{"confirm":true,"version_id":0}`, []string{"1", "paid"}, false, 400}, {"POST", `{"confirm":true,"version_id":1}`, []string{"1", "paid"}, false, 503}}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, "/", strings.NewReader(c.body))
		if c.cross {
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		w := httptest.NewRecorder()
		h.markInterestPaid(w, r, 1, 2, c.path)
		if w.Code != c.want {
			t.Fatalf("%s %v: %d != %d", c.method, c.path, w.Code, c.want)
		}
	}
	for _, status := range []string{"paid", "unpaid", ""} {
		f, e := parseAnalyticsFilters(url.Values{"interest_payment": {status}}, time.Now())
		if e != nil || f.values().Get("interest_payment") != status {
			t.Fatal("payment filter roundtrip")
		}
	}
	if _, e := parseAnalyticsFilters(url.Values{"interest_payment": {"unknown"}}, time.Now()); e == nil {
		t.Fatal("invalid payment filter accepted")
	}
}
