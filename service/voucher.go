package voucher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ingemar0720/voucher-pool/database"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type VoucherSrv struct {
	DB  *sqlx.DB
	Ctx context.Context
}

type VoucherRequest struct {
	Code  string `json:"code"`
	Email string `json:"email"`
}

type DiscountResponse struct {
	Discount float32 `json:"discount"`
}

func New() (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", "host=db user=user dbname=postgres password=mysecretpassword sslmode=disable")
	if err != nil {
		return &sqlx.DB{}, fmt.Errorf("fail to connect to db, error: %v", err)
	}
	return db, nil
}

// receives a Voucher Code and Email and validates the Voucher Code. In Case it is valid, return the Percentage Discount and set the date of usage
func (srv *VoucherSrv) VoucherHanlder(w http.ResponseWriter, r *http.Request) {
	vr := VoucherRequest{}
	err := json.NewDecoder(r.Body).Decode(&vr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	t, err := database.ValidateVoucher(srv.Ctx, vr.Email, vr.Code, srv.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// column used_at not null, this voucher has been redeemed
	if t.Valid {
		http.Error(w, "this voucher has been redeemed", http.StatusBadRequest)
		return
	} else {
		discount, err := database.SetVoucherUsageAndGetDiscount(srv.Ctx, vr.Code, srv.DB)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		discountResp := DiscountResponse{
			Discount: discount,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(discountResp)
		return
	}
}
