package voucher

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/mail"
	"time"

	"github.com/ingemar0720/voucher-pool/dbmodel"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type VoucherSrv struct {
	DB  *sqlx.DB
	Ctx context.Context
}

type ValidateRequest struct {
	Code  string `json:"code"`
	Email string `json:"email"`
}

type ListRequest struct {
	Email string `json:"email"`
}

type GenerateRequest struct {
	Email     string    `json:"email"`
	OfferName string    `json:"offer_name"`
	Discount  float32   `json:"discount"`
	Expiry    time.Time `json:"expiry"`
}

type ValidateResponse struct {
	Discount float32 `json:"discount"`
}

type GenerateResponse struct {
	Code string `json:"code"`
}

type GetResponse struct {
	Code      string `json:"code"`
	OfferName string `json:"offer_name"`
}

func New() (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", "postgres://user:mysecretpassword@db:5432/postgres?sslmode=disable")
	if err != nil {
		return &sqlx.DB{}, fmt.Errorf("fail to connect to db, error: %v", err)
	}
	return db, nil
}

// receives a Voucher Code and Email and validates the Voucher Code. In Case it is valid, return the Percentage Discount and set the date of usage
func (srv *VoucherSrv) ValidateHanlder(w http.ResponseWriter, r *http.Request) {
	vr := ValidateRequest{}
	err := json.NewDecoder(r.Body).Decode(&vr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = mail.ParseAddress(vr.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	t, err := dbmodel.ValidateVoucher(srv.Ctx, vr.Email, vr.Code, srv.DB)
	if err != nil {
		if err.Error() == "voucher expired" {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// column used_at not null, this voucher has been redeemed
	if t.Valid {
		http.Error(w, "this voucher has been redeemed", http.StatusBadRequest)
		return
	} else {
		discount, err := dbmodel.SetVoucherUsageAndGetDiscount(srv.Ctx, vr.Code, srv.DB)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		validateResp := ValidateResponse{
			Discount: discount,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(validateResp)
		return
	}
}

func (srv *VoucherSrv) GenerateHanlder(w http.ResponseWriter, r *http.Request) {
	gr := GenerateRequest{}
	err := json.NewDecoder(r.Body).Decode(&gr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//validate input
	_, err = mail.ParseAddress(gr.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if gr.Discount <= 0 || gr.Discount > 100.00 {
		http.Error(w, "discount shall bigger than 0 or less than 100.00", http.StatusBadRequest)
		return
	}
	if gr.Expiry.Before(time.Now()) {
		http.Error(w, "expiry date shall be in the future", http.StatusBadRequest)
		return
	}

	//generate code
	code := RandStringBytes(8)
	if err := dbmodel.GenerateVoucher(srv.Ctx, gr.Email, gr.OfferName, code, gr.Expiry, gr.Discount, srv.DB); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(GenerateResponse{Code: code})
}

//https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func RandStringBytes(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func (srv *VoucherSrv) GetValidVouchers(w http.ResponseWriter, r *http.Request) {
	lr := ListRequest{}
	err := json.NewDecoder(r.Body).Decode(&lr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//validate input
	_, err = mail.ParseAddress(lr.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	codes, offerNames, err := dbmodel.GetVouchers(srv.Ctx, lr.Email, srv.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(codes) != len(offerNames) {
		if err != nil {
			http.Error(w, "number of column of voucher code not equal to number of columns of specail_offer name", http.StatusInternalServerError)
			return
		}
	}
	var vouchers []GetResponse
	l := len(codes)
	for i := 0; i < l; i++ {
		vouchers = append(vouchers, GetResponse{Code: codes[i], OfferName: offerNames[i]})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(vouchers)
}
