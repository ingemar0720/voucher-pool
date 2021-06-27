# voucher-pool

## Task and description

The repo is to create a voucher pool backend service can be used by customers to get discounts on website. This service can generate voucher, validate given voucher and list all vouchers belongs to certain user. It's pure backend microservice running with docker-compose in local. There is no UI, cluster deployment, monitoring.

### Functionalities

- given customer email, special offer name, specail offer discount and voucher expiry, the generate API shall generate unique special offer and return associated unique voucher code.
- given a unique voucher code and user email, the validate API shall validates the voucher code. In case it is valid, return the Percentage Discount and set the date of usage to now.
- given a customer email, the list API shall return all its valid voucher code with the Name of the speical offer

### API

- generate API: POST `localhost:5000/vouchers/generate` to generate voucher with body below

```
{
    "email":"customer1@gmail.com",
    "offer_name":"KOI",
    "discount":22.1,
    "expiry":"2022-04-21T18:25:43-05:00"
}
```

- validate API: POST `localhost:5000/vouchers/validate` to validate voucher with body in JSNO format, code must be matched with the response from generate endpoint

```
{
    "code":"zxIsYkFC",
    "email":"customer1@gmail.com"
}
```

- list API: GET `localhost:5000/vouchers` to get list of valid vouchers for a given user email.

```
{
    "email":"customer1@gmail.com"
}
```

### Commands for services

- Run service: `docker-compose up go`, go service will run on port 5000, postgres db will run on port 5432
- Run test: `docker-compose up gotest`, `dbmodel/voucher_test.go` is unit test which mocks postgres and `service/voucher_test.go` is integration test running with test database.
- Seeding for go service: This service doesn't provide sign up API, so `docker-compose up dbseed` will seed 10 customers into DB before the go service start.

### Tech decision

- Choose postgres as the problem statement has a couple of stable relationships and schema seems to be fixed.
- Use integration test in service/voucer_test.go as it contains most of business logic. It's better to use real DB to test.

### Something to be improved

- Add DB connection management and retry.
- Pregenerate voucher code and put into memory cache. If the traffic is too high, we don't need to spend compute on random code generation.
- Migrate used voucher record into differnt table to reduce the future query effort. Can also do a regular cleanup for that specific table to reduce storage cost.
- Review error handling of database operation, current code use some customised error msg and shall be refactored.
