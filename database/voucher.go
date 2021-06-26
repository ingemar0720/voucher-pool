package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

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
	tx, err := db.BeginTx(ctx, nil)
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
