package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type DBModelSpecialOffer struct {
	Name     string  `json:"name" db:"name"`
	Discount float32 `json:"discount" db:"discount"`
}

type DBModelVoucher struct {
	Code           string       `json:"code" db:"code"`
	CustomerID     uint64       `json:"customer_id" db:"customer_id"`
	SpecialOfferID uint64       `json:"special_offer_id" db:"special_offer_id"`
	ExpiryDate     time.Time    `json:"expired_at" db:"expired_at"`
	UsedDate       sql.NullTime `json:"used_at" db:"used_at"`
}

func ValidateVoucher(ctx context.Context, email, code string, db *sqlx.DB) (sql.NullTime, error) {
	rows, err := db.QueryContext(ctx, "SELECT vo.used_at FROM customers cus inner JOIN vouchers vo ON cus.id=vo.customer_id WHERE cus.email=$1 AND vo.code=$2", email, code)
	if err != nil {
		return sql.NullTime{}, errors.Wrapf(err, "fail to query used_at from table vouchers")
	}
	defer rows.Close()
	t := sql.NullTime{}
	if rows.Next() {
		err := rows.Scan(&t)
		if err != nil {
			return sql.NullTime{}, errors.Wrapf(err, "fail to query used_at from vouchers table")
		}
	}
	return t, nil
}

func SetVoucherUsageAndGetDiscount(ctx context.Context, code string, db *sqlx.DB) (float32, error) {
	rows, err := db.QueryContext(ctx, "SELECT so.discount FROM special_offers so INNER JOIN vouchers vo ON so.id=vo.special_offer_id WHERE vo.code=$1", code)
	if err != nil {
		return 0, errors.Wrapf(err, "fail to query discount from table special_offers")
	}
	var discount float32
	if rows.Next() {
		err := rows.Scan(&discount)
		if err != nil {
			return 0, errors.Wrapf(err, "fail to query discount from table special_offers")
		}
	}
	rows.Close()
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, errors.Wrapf(err, "fail to setup date of usage")
	}
	_, err = tx.ExecContext(ctx, "UPDATE vouchers SET used_at=$1 WHERE code=$2", time.Now().Format(time.RFC3339), code)
	if err != nil {
		err = fmt.Errorf("fail to setup date of usage, error %v", err)
		if err1 := tx.Rollback(); err1 != nil {
			return 0, errors.Wrapf(err1, "fail to rollback date of usage, update error %v", err)
		}
		return 0, err
	}
	err = tx.Commit()
	if err != nil {
		return 0, errors.Wrapf(err, "fail to commit update of date of usage")
	}
	return discount, nil
}

func GenerateVoucher(ctx context.Context, email, offerName, code string, expiry time.Time, discount float32, db *sqlx.DB) error {
	// query customer id
	rows, err := db.QueryContext(ctx, "SELECT id FROM customers WHERE email=$1", email)
	if err != nil {
		return errors.Wrapf(err, "fail to find customer with email %v", email)
	}
	var customerID uint64
	if rows.Next() {
		err := rows.Scan(&customerID)
		if err != nil {
			return errors.Wrapf(err, "fail to query discount from table special_offers")
		}
	}
	rows.Close()
	if customerID == 0 {
		return errors.New("customerID shall not be 0")
	}

	// upsert special_offer, so that we could update the discount of the special_offers
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "fail to setup date of usage")
	}

	offer := DBModelSpecialOffer{
		Name:     offerName,
		Discount: discount,
	}
	var offerID uint64
	offerRows, err := tx.NamedQuery(`INSERT INTO special_offers (name, discount) VALUES (:name, :discount)
										ON CONFLICT (name) DO UPDATE SET discount=EXCLUDED.discount
										RETURNING id`, offer)
	if err != nil {
		if err1 := tx.Rollback(); err1 != nil {
			return errors.Wrapf(err1, "fail to rollback insert to specail_offer table, insert error %v", err)
		}
		return errors.Wrapf(err, "fail to insert into table special_offers")
	}
	if offerRows.Next() {
		offerRows.Scan(&offerID)
	}
	offerRows.Close()
	if offerID == 0 {
		return errors.New("offerID shall not be 0")
	}
	// insert voucher
	voucher := DBModelVoucher{
		Code:           code,
		CustomerID:     customerID,
		SpecialOfferID: offerID,
		ExpiryDate:     expiry,
		UsedDate:       sql.NullTime{Valid: false},
	}
	_, err = tx.NamedExecContext(ctx, "INSERT INTO vouchers (code, customer_id, special_offer_id, expired_at, used_at) VALUES (:code, :customer_id, :special_offer_id, :expired_at, :used_at)", voucher)
	if err != nil {
		if err1 := tx.Rollback(); err1 != nil {
			return errors.Wrapf(err1, "fail to rollback insert to voucher table, insert error %v", err)
		}
		return errors.Wrapf(err, "fail to insert to voucher table")
	}
	return tx.Commit()
}
