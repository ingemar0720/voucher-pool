package main

import (
	"fmt"
	"log"

	voucher "github.com/ingemar0720/voucher-pool/service"
	"github.com/pkg/errors"
)

func main() {
	db, err := voucher.New()
	if err != nil {
		log.Fatal(errors.Wrapf(err, "fail to init a DB instance"))
	}
	tx, err := db.Beginx()
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		_, err = tx.Exec("INSERT INTO customers (name, email) VALUES ($1, $2)", fmt.Sprintf("customer %v", i), fmt.Sprintf("customer%v@gmail.com", i))
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
	}
	tx.Commit()
}
