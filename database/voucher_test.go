package database

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
