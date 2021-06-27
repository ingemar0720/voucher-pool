package dbmodel

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func setupSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	return db, mock
}

func TestValidateVoucher(t *testing.T) {
	db, mock := setupSQLMock(t)
	defer db.Close()
	fixtureEmail := "test@gmail.com"
	fixtureCode := "code"
	tests := []struct {
		name       string
		givenEmail string
		givenCode  string
		want       sql.NullTime
		wantErr    bool
	}{
		{
			name:       "used_at is null",
			givenEmail: fixtureEmail,
			givenCode:  fixtureCode,
			want:       sql.NullTime{Valid: false},
			wantErr:    false,
		},
		{
			name:       "used_at is not null",
			givenEmail: fixtureEmail,
			givenCode:  fixtureCode,
			want:       sql.NullTime{Valid: true, Time: time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local)},
			wantErr:    false,
		},
		{
			name:       "db error",
			givenEmail: fixtureEmail,
			givenCode:  fixtureCode,
			want:       sql.NullTime{},
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantErr {
				mock.ExpectQuery("SELECT (.+) FROM customers cus inner JOIN vouchers vo ON cus.id=vo.customer_id WHERE (.+)").WithArgs(fixtureEmail, fixtureCode).WillReturnRows(sqlmock.NewRows([]string{"used_at"}).AddRow(tt.want))
			} else {
				mock.ExpectQuery("SELECT (.+) FROM customers cus inner JOIN vouchers vo ON cus.id=vo.customer_id WHERE (.+)").WithArgs(fixtureEmail, fixtureCode).WillReturnError(errors.New("error"))
			}
			got, err := ValidateVoucher(context.Background(), tt.givenEmail, tt.givenCode, sqlx.NewDb(db, "sqlmock"))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVoucher() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateVoucher() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetVoucherUsageAndGetDiscount(t *testing.T) {
	db, mock := setupSQLMock(t)
	defer db.Close()
	fixtureCode := "code"
	tests := []struct {
		name          string
		givenCode     string
		givenUsedDate sql.NullTime
		want          float32
		queryErr      bool
		updateErr     bool
	}{
		{
			name:          "get discount value",
			givenCode:     fixtureCode,
			givenUsedDate: sql.NullTime{Valid: true, Time: time.Now()},
			want:          50.1,
			queryErr:      false,
			updateErr:     false,
		},
		{
			name:          "query error",
			givenCode:     fixtureCode,
			givenUsedDate: sql.NullTime{Valid: true, Time: time.Now()},
			want:          0,
			queryErr:      true,
			updateErr:     false,
		},
		{
			name:          "update error",
			givenCode:     fixtureCode,
			givenUsedDate: sql.NullTime{Valid: true, Time: time.Now()},
			want:          0,
			queryErr:      false,
			updateErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.queryErr {
				mock.ExpectQuery("SELECT (.+) FROM special_offers so INNER JOIN vouchers vo ON so.id=vo.special_offer_id WHERE (.+)").WithArgs(fixtureCode).WillReturnRows(sqlmock.NewRows([]string{"discount"}).AddRow(50.1))
			} else {
				mock.ExpectQuery("SELECT (.+) FROM special_offers so INNER JOIN vouchers vo ON so.id=vo.special_offer_id WHERE (.+)").WithArgs(fixtureCode).WillReturnError(errors.New("error"))
			}
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE vouchers SET (.+) WHERE (.+)").WithArgs(tt.givenUsedDate.Time.Format(time.RFC3339), tt.givenCode).WillReturnResult(sqlmock.NewResult(1, 1))
			if !tt.updateErr {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}

			got, err := SetVoucherUsageAndGetDiscount(context.Background(), tt.givenCode, sqlx.NewDb(db, "sqlmock"))
			if tt.queryErr {
				if err == nil {
					t.Errorf("SetVoucherUsageAndGetDiscount() error = %v, queryErr %v", err, tt.queryErr)
					return
				}
			}
			if tt.updateErr {
				if err == nil {
					t.Errorf("SetVoucherUsageAndGetDiscount() error = %v, updateErr %v", err, tt.updateErr)
					return
				}
			}
			if err == nil {
				assert.EqualValues(t, tt.updateErr, false)
				assert.EqualValues(t, tt.queryErr, false)
			}
			if got != tt.want {
				t.Errorf("SetVoucherUsageAndGetDiscount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateVoucher(t *testing.T) {
	db, mock := setupSQLMock(t)
	defer db.Close()
	fixtureEmail := "test@gmail.com"
	fixtureOfferName := "apple_store"
	fixtureVoucherCode := "abcd"
	fixtureExpiry := time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local)
	fixtureDiscount := float32(88.8)

	tests := []struct {
		name                 string
		givenEmail           string
		givenOfferName       string
		givenVoucherCode     string
		givenExpiry          time.Time
		givenDiscount        float32
		wantQueryCustomerErr bool
		wantUpsertOfferErr   bool
		wantInsertVoucherErr bool
	}{
		{
			name:             "generate voucher record successfully",
			givenEmail:       fixtureEmail,
			givenOfferName:   fixtureOfferName,
			givenVoucherCode: fixtureVoucherCode,
			givenExpiry:      fixtureExpiry,
			givenDiscount:    fixtureDiscount,
		},
		{
			name:                 "fail to query customer_id",
			givenEmail:           fixtureEmail,
			givenOfferName:       fixtureOfferName,
			givenVoucherCode:     fixtureVoucherCode,
			givenExpiry:          fixtureExpiry,
			givenDiscount:        fixtureDiscount,
			wantQueryCustomerErr: true,
		},
		{
			name:               "fail to upsert offer",
			givenEmail:         fixtureEmail,
			givenOfferName:     fixtureOfferName,
			givenVoucherCode:   fixtureVoucherCode,
			givenExpiry:        fixtureExpiry,
			givenDiscount:      fixtureDiscount,
			wantUpsertOfferErr: true,
		},
		{
			name:                 "fail to insert voucher",
			givenEmail:           fixtureEmail,
			givenOfferName:       fixtureOfferName,
			givenVoucherCode:     fixtureVoucherCode,
			givenExpiry:          fixtureExpiry,
			givenDiscount:        fixtureDiscount,
			wantInsertVoucherErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantQueryCustomerErr {
				mock.ExpectQuery("SELECT (.+) FROM customers WHERE (.+)").WithArgs(tt.givenEmail).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow(1))
			} else {
				mock.ExpectQuery("SELECT (.+) FROM customers WHERE (.+)").WithArgs(tt.givenEmail).WillReturnError(errors.New("error"))
			}
			mock.ExpectBegin()
			if !tt.wantUpsertOfferErr {
				mock.ExpectQuery("INSERT INTO special_offers (.+) VALUES (.+) ON CONFLICT (.+) DO UPDATE SET (.+) RETURNING id").WithArgs(tt.givenOfferName, tt.givenDiscount).WillReturnRows(sqlmock.NewRows([]string{"used_at"}).AddRow(1))
			} else {
				mock.ExpectQuery("INSERT INTO special_offers (.+) VALUES (.+) ON CONFLICT (.+) DO UPDATE SET (.+) RETURNING id").WithArgs(tt.givenOfferName, tt.givenDiscount).WillReturnError(errors.New("error"))
				mock.ExpectRollback()
			}

			if !tt.wantInsertVoucherErr {
				mock.ExpectExec("INSERT INTO vouchers (.+) VALUES (.+)").WithArgs(tt.givenVoucherCode, 1, 1, tt.givenExpiry, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
			} else {
				mock.ExpectExec("INSERT INTO vouchers (.+) VALUES (.+)").WithArgs(tt.givenVoucherCode, 1, 1, tt.givenExpiry, sqlmock.AnyArg()).WillReturnError(errors.New("error"))
			}
			if tt.wantUpsertOfferErr || tt.wantInsertVoucherErr {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			err := GenerateVoucher(context.Background(), tt.givenEmail, tt.givenOfferName, tt.givenVoucherCode, tt.givenExpiry, tt.givenDiscount, sqlx.NewDb(db, "sqlmock"))
			if tt.wantQueryCustomerErr {
				assert.NotNil(t, err)
			}
			if tt.wantUpsertOfferErr {
				assert.NotNil(t, err)
			}
			if tt.wantUpsertOfferErr {
				assert.NotNil(t, err)
			}
			if !tt.wantQueryCustomerErr && !tt.wantUpsertOfferErr && !tt.wantInsertVoucherErr {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetVouchers(t *testing.T) {
	db, mock := setupSQLMock(t)
	defer db.Close()
	fixtureEmail := "test@gmail.com"
	fixtureCode := []string{"abcd", "defg"}
	fixtureOfferName := []string{"apple_store", "7-11"}
	tests := []struct {
		name                 string
		givenEmail           string
		wantVoucerCodes      []string
		wantOffNames         []string
		wantQueryCustomerErr bool
		wantQueryVoucherErr  bool
	}{
		{
			name:                 "get valid vouchers successfully",
			givenEmail:           fixtureEmail,
			wantVoucerCodes:      fixtureCode,
			wantOffNames:         fixtureOfferName,
			wantQueryCustomerErr: false,
			wantQueryVoucherErr:  false,
		},
		{
			name:                 "fail to query customer",
			givenEmail:           fixtureEmail,
			wantVoucerCodes:      []string{},
			wantOffNames:         []string{},
			wantQueryCustomerErr: true,
			wantQueryVoucherErr:  false,
		},
		{
			name:                 "fail to query voucher",
			givenEmail:           fixtureEmail,
			wantVoucerCodes:      []string{},
			wantOffNames:         []string{},
			wantQueryCustomerErr: false,
			wantQueryVoucherErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantQueryCustomerErr {
				mock.ExpectQuery("SELECT (.+) FROM customers WHERE (.+)").WithArgs(tt.givenEmail).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow(1))
			} else {
				mock.ExpectQuery("SELECT (.+) FROM customers WHERE (.+)").WithArgs(tt.givenEmail).WillReturnError(errors.New("error"))
			}

			if !tt.wantQueryVoucherErr {
				mock.ExpectQuery("SELECT (.+) FROM vouchers AS vo INNER JOIN special_offers AS so ON vo.special_offer_id=so.id WHERE (.+)").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"code", "name"}).AddRow("abcd", "apple_store").AddRow("defg", "7-11"))
			} else {
				mock.ExpectQuery("SELECT (.+) FROM vouchers AS vo INNER JOIN special_offers AS so ON vo.special_offer_id=so.id WHERE (.+)").WithArgs(1).WillReturnError(errors.New("error"))
			}

			got, got1, err := GetVouchers(context.Background(), tt.givenEmail, sqlx.NewDb(db, "sqlmock"))

			if !tt.wantQueryCustomerErr && !tt.wantQueryVoucherErr {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
			if !reflect.DeepEqual(got, tt.wantVoucerCodes) {
				t.Errorf("GetVouchers() got = %v, want %v", got, tt.wantVoucerCodes)
			}
			if !reflect.DeepEqual(got1, tt.wantOffNames) {
				t.Errorf("GetVouchers() got1 = %v, want %v", got1, tt.wantOffNames)
			}
		})
	}
}
