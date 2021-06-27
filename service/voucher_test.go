package voucher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// var Cleaner = dbcleaner.New()

type TestSuite struct {
	suite.Suite
	srv *VoucherSrv
}

// setup test suite, prepare test db, db cleaner
func (suite *TestSuite) SetupSuite() {
	fmt.Println("setup suite")
	db, err := sqlx.Connect("postgres", "postgres://user:mysecretpassword@db:5432/postgres_test?sslmode=disable")
	if err != nil {
		assert.Fail(suite.T(), "setup test DB fail", err)
	}

	suite.srv = &VoucherSrv{DB: db, Ctx: context.Background()}
}

func (suite *TestSuite) TearDownSuite() {
	fmt.Println("teardown suite")
}

func (suite *TestSuite) BeforeTest(suiteName, testName string) {
	fmt.Println("before test")
	// seed 2 user
	tx, err := suite.srv.DB.Beginx()
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		_, err = tx.Exec("INSERT INTO customers (name, email) VALUES ($1, $2)", fmt.Sprintf("customer %v", i), fmt.Sprintf("customer%v@gmail.com", i))
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
	}
	tx.Commit()
}

func (suite *TestSuite) AfterTest(suiteName, testName string) {
	fmt.Println("after test")
	tx, err := suite.srv.DB.Beginx()
	if err != nil {
		log.Fatal(err)
	}
	_, err = tx.Exec("TRUNCATE TABLE customers RESTART IDENTITY CASCADE")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}
	_, err = tx.Exec("TRUNCATE TABLE special_offers RESTART IDENTITY CASCADE")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}
	_, err = tx.Exec("TRUNCATE TABLE vouchers RESTART IDENTITY CASCADE")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}
	tx.Commit()
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func httpTestHelper(method, url string, body io.Reader, srv *VoucherSrv, f func(http.ResponseWriter, *http.Request)) (*http.Response, []byte) {
	req := httptest.NewRequest(method, url, body)
	w := httptest.NewRecorder()
	f(w, req)
	resp := w.Result()
	respBody, _ := ioutil.ReadAll(resp.Body)
	return resp, respBody
}

func (suite *TestSuite) TestValidateHanlder() {
	resp, body := httpTestHelper("POST", "http://vouchers/validate", nil, suite.srv, suite.srv.ValidateHanlder)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "EOF\n", string(body))

	resp, body = httpTestHelper("POST", "http://vouchers/validate", bytes.NewBuffer([]byte(`{"email": "invalid_email", "code": "abc"}`)), suite.srv, suite.srv.ValidateHanlder)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "mail: missing '@' or angle-addr\n", string(body))

	// seed to prepare the db test
	tx, err := suite.srv.DB.BeginTx(suite.srv.Ctx, nil)
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	_, err = tx.Exec("INSERT INTO special_offers (name, discount) VALUES ($1, $2)", "apple_store", 38.5)
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	_, err = tx.Exec("INSERT INTO vouchers (code, customer_id, special_offer_id, expired_at) VALUES ($1, $2, $3, $4)", "abc", 1, 1, time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	err = tx.Commit()
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}

	//test the happy case
	resp, body = httpTestHelper("POST", "http://vouchers/validate", bytes.NewBuffer([]byte(`{"email": "customer0@gmail.com", "code": "abc"}`)), suite.srv, suite.srv.ValidateHanlder)
	assert.EqualValues(suite.T(), http.StatusCreated, resp.StatusCode)
	assert.EqualValues(suite.T(), "{\"discount\":38.5}\n", string(body))

	//test again shall get redeemed error
	resp, body = httpTestHelper("POST", "http://vouchers/validate", bytes.NewBuffer([]byte(`{"email": "customer0@gmail.com", "code": "abc"}`)), suite.srv, suite.srv.ValidateHanlder)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "this voucher has been redeemed\n", string(body))
}

func (suite *TestSuite) TestGenerateHandler() {
	resp, body := httpTestHelper("POST", "http://vouchers/validate", nil, suite.srv, suite.srv.GenerateHanlder)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "EOF\n", string(body))

	resp, body = httpTestHelper("POST", "http://vouchers/validate", bytes.NewBuffer([]byte(`{"email": "invalid_email", "code": "abc"}`)), suite.srv, suite.srv.GenerateHanlder)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "mail: missing '@' or angle-addr\n", string(body))

	tomorrow := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	sqltext := `{"email": "test1@gmail.com", "offer_name": "apple_store", "discount": 101.0, "expiry": "` + tomorrow + `"}`
	resp, body = httpTestHelper("POST", "http://vouchers/validate", bytes.NewBuffer([]byte(sqltext)), suite.srv, suite.srv.GenerateHanlder)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "discount shall bigger than 0 or less than 100.00\n", string(body))

	yesterday := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	sqltext = `{"email": "test1@gmail.com", "offer_name": "apple_store", "discount": 54.30, "expiry": "` + yesterday + `"}`
	resp, body = httpTestHelper("POST", "http://vouchers/validate", bytes.NewBuffer([]byte(sqltext)), suite.srv, suite.srv.GenerateHanlder)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "expiry date shall be in the future\n", string(body))

	// seed to prepare the db test
	tx, err := suite.srv.DB.BeginTx(suite.srv.Ctx, nil)
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	_, err = tx.Exec("INSERT INTO special_offers (name, discount) VALUES ($1, $2)", "apple_store", 38.5)
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	_, err = tx.Exec("INSERT INTO vouchers (code, customer_id, special_offer_id, expired_at) VALUES ($1, $2, $3, $4)", "abc", 1, 1, time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	err = tx.Commit()
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}

	sqltext = `{"email": "customer0@gmail.com", "offer_name": "apple_store", "discount": 54.30, "expiry": "` + tomorrow + `"}`
	resp, body = httpTestHelper("POST", "http://vouchers/validate", bytes.NewBuffer([]byte(sqltext)), suite.srv, suite.srv.GenerateHanlder)
	assert.EqualValues(suite.T(), http.StatusCreated, resp.StatusCode)
	assert.EqualValues(suite.T(), 20, len(string(body)))
	assert.EqualValues(suite.T(), true, strings.Contains(string(body), `{"code":`))

}

func (suite *TestSuite) TestGetValidVouchers() {
	resp, body := httpTestHelper("GET", "http://vouchers", nil, suite.srv, suite.srv.GetValidVouchers)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "EOF\n", string(body))

	resp, body = httpTestHelper("GET", "http://vouchers", bytes.NewBuffer([]byte(`{"email": "invalid_email", "code": "abc"}`)), suite.srv, suite.srv.GetValidVouchers)
	assert.EqualValues(suite.T(), http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(suite.T(), "mail: missing '@' or angle-addr\n", string(body))

	// seed to prepare the db test
	tx, err := suite.srv.DB.BeginTx(suite.srv.Ctx, nil)
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	// insert 2 special_offer records
	_, err = tx.Exec("INSERT INTO special_offers (name, discount) VALUES ($1, $2)", "apple_store", 38.5)
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	_, err = tx.Exec("INSERT INTO special_offers (name, discount) VALUES ($1, $2)", "KOI", 76.5)
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	// insert 2 voucher records with same customer ID but different offer ID, only the second one is not redeemed
	_, err = tx.Exec("INSERT INTO vouchers (code, customer_id, special_offer_id, used_at, expired_at) VALUES ($1, $2, $3, $4, $5)", "abc", 1, 1, time.Date(2021, time.May, 1, 0, 0, 0, 0, time.Local), time.Date(2021, time.August, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	_, err = tx.Exec("INSERT INTO vouchers (code, customer_id, special_offer_id, expired_at) VALUES ($1, $2, $3, $4)", "def", 1, 2, time.Date(2021, time.September, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	err = tx.Commit()
	if err != nil {
		assert.FailNow(suite.T(), err.Error())
	}
	resp, body = httpTestHelper("GET", "http://vouchers", bytes.NewBuffer([]byte(`{"email": "customer0@gmail.com"}`)), suite.srv, suite.srv.GetValidVouchers)
	assert.EqualValues(suite.T(), http.StatusCreated, resp.StatusCode)
	// assert to get second voucher code and offer name
	assert.EqualValues(suite.T(), `[{"code":"def","offer_name":"KOI"}]`+"\n", string(body))
}
