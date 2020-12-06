package grtest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	errs "github.com/pkg/errors"

	"github.com/monstercat/golib/expectm"
	"github.com/monstercat/golib/request"
)

type M map[string]interface{}

type RouteTest struct {
	Body              interface{}
	BodyShouldHave    []string
	BodyShouldNotHave []string
	Cookies           map[string]string
	ExpectedBody      string
	ExpectedCookies   *expectm.ExpectedM
	ExpectedStatus    int
	ExpectedM         *expectm.ExpectedM
	GetURL            func(path string) string
	HideResponseBody  bool // If true, it won't print out the ENTIRE BODY as html to the console on a fail
	Method            string
	ModifyParams      func(params *request.Params) error
	Name              string
	Only              bool
	Path              string
	Preflight         func() error
	Query             *M
	Response          M
	NilResponse       bool
	URL               string
}

func RunTests(tests []*RouteTest) error {
	toRun := make([]*RouteTest, 0)
	var hasOnly bool
	for _, test := range tests {
		if test.Only {
			hasOnly = true
			toRun = append(toRun, test)
		}
	}
	if !hasOnly {
		toRun = tests
	} else {
		fmt.Printf("Excluding %d tests because of Only flag\n", len(tests)-len(toRun))
	}
	for i, test := range toRun {
		if err := test.Run(); err != nil {
			name := ""
			if test.Name != "" {
				name = " " + test.Name
			}
			return errs.Errorf("[%d]%s %s", i, name, err.Error())
		}
	}
	return nil
}

func (t *RouteTest) Run() error {
	if t.Preflight != nil {
		if err := t.Preflight(); err != nil {
			return err
		}
	}

	method := t.Method
	if method == "" {
		method = http.MethodGet
	}

	requestURL := ""
	if t.URL == "" {
		if t.GetURL == nil {
			return errs.New("no test URL and no GetURL func")
		} else {
			t.URL = t.GetURL(t.Path)
		}
	} else {
		requestURL = t.URL
	}
	params := request.Params{
		Url:    requestURL,
		Method: method,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	if t.Query != nil {
		vals := url.Values{}
		for k, v := range *t.Query {
			vals.Add(k, fmt.Sprintf("%v", v))
		}
		params.Url += "?" + vals.Encode()
	}

	if t.ModifyParams != nil {
		if err := t.ModifyParams(&params); err != nil {
			return err
		}
	}
	if err := request.Request(&params, t.Body, &t.Response); err != nil {
		if t.ExpectedStatus < 300 {
			fmt.Println("Request error", err)
		}
	}

	testInfo := fmt.Sprintf(`
Response: %s
URL: %s
Method: %s
Payload: %s`, params.ResponseBody, params.Url, params.Method, t.Body)

	if !t.NilResponse {
		if params.Response == nil {
			fmt.Println("NIL")
			return errs.Errorf("params.Response is nil" + testInfo)
		} else {
			if params.Response.StatusCode != t.ExpectedStatus {
				return errs.Errorf("Expected status %d but got %d.%s", t.ExpectedStatus, params.Response.StatusCode, testInfo)
			}
		}
	}

	if t.ExpectedBody != "" {
		if params.ResponseBody != t.ExpectedBody {
			return errs.Errorf("Body did not match\nExpected: %s\n  Found: %s", t.ExpectedBody, params.ResponseBody)
		}
	}

	printedResponseBody := params.ResponseBody

	if t.HideResponseBody {
		printedResponseBody = ""
	}
	if len(t.BodyShouldHave) > 0 {
		for i, should := range t.BodyShouldHave {
			if !strings.Contains(params.ResponseBody, should) {
				return errs.Errorf("Body did not contain [%d]: '%s'\nBody: %s", i, should, printedResponseBody)
			}
		}
	}

	if len(t.BodyShouldNotHave) > 0 {
		for i, shouldNot := range t.BodyShouldNotHave {
			if strings.Contains(params.ResponseBody, shouldNot) {
				return errs.Errorf("Body should NOT contain [%d]: '%s'\nBody: %s", i, shouldNot, printedResponseBody)
			}
		}
	}

	if t.ExpectedM != nil {
		err := expectm.CheckJSONString(params.ResponseBody, t.ExpectedM)
		if err != nil {
			return errs.Wrap(err, testInfo)
		}
	}

	t.Cookies = make(map[string]string)
	if params.Response != nil {
		cookies := params.Response.Cookies()
		for _, v := range cookies {
			t.Cookies[v.Name] = v.Value
		}
	}
	if t.ExpectedCookies != nil {
		err := expectm.CheckJSON(t.Cookies, t.ExpectedCookies)
		if err != nil {
			return errs.Wrap(err, testInfo)
		}
	}

	return nil
}

func (t *RouteTest) Apply(tests []*RouteTest) []*RouteTest {
	for _, test := range tests {
		if test.Path == "" {
			test.Path = t.Path
		}
		if test.Method == "" {
			test.Method = t.Method
		}
		if test.ModifyParams == nil {
			test.ModifyParams = t.ModifyParams
		}
		if test.ExpectedStatus == 0 {
			test.ExpectedStatus = t.ExpectedStatus
		}
		if test.ExpectedM == nil {
			test.ExpectedM = t.ExpectedM
		}
		if test.Query == nil {
			test.Query = t.Query
		}
		if test.Preflight == nil {
			test.Preflight = t.Preflight
		}
		if test.ExpectedCookies == nil {
			test.ExpectedCookies = t.ExpectedCookies
		}
		if len(test.BodyShouldHave) == 0 {
			test.BodyShouldHave = t.BodyShouldHave
		}
		if len(test.BodyShouldNotHave) == 0 {
			test.BodyShouldNotHave = t.BodyShouldNotHave
		}
		if test.ExpectedBody == "" {
			test.ExpectedBody = t.ExpectedBody
		}
	}
	return tests
}
