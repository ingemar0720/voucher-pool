package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	voucher "github.com/ingemar0720/voucher-pool/service"
	"github.com/pkg/errors"
)

func main() {
	fmt.Println("hellow voucher service")
	db, err := voucher.New()
	if err != nil {
		log.Fatal(errors.Wrapf(err, "fail to init a DB instance"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := voucher.VoucherSrv{DB: db, Ctx: ctx}
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Logger)
	r.Post("/vouchers/validate", srv.ValidateHanlder)
	r.Post("/vouchers/generate", srv.GenerateHanlder)
	r.Get("/vouchers", srv.GetValidVouchers)
	log.Fatal(http.ListenAndServe(":5000", r))
}
