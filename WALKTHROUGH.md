# Service walkthrough

These commands are the stage 1 start command from `stage-1/RUN.md` and the request shapes from the official stage 1 specification. They are not a recording. No room video of this run was found, and the Phoenix demo was not used.

New accounts start at balance `0`, so the payment below uses the specification's seeded fixture. Ada's seeded balance is `10000` minor units. The payment amount `1500` is the specification's example.

From `stage-1/`:

```sh
docker build -t pocketful-stage1 . && docker run --rm -p 8080:8080 -e PORT=8080 pocketful-stage1
```

In another shell:

```sh
curl -sS -D - http://127.0.0.1:8080/health
```

The specification says `GET /health` returns `200` and `{"status":"ok"}` once the service can accept requests.

```sh
curl -sS -D - http://127.0.0.1:8080/_test/reset \
  -H 'Content-Type: application/json' \
  -d '{"currency":"EUR","minor_units":2,"users":[{"id":"u_ada","email":"ada@example.com","password":"correct horse","display_name":"Ada","handle":"ada","balance":10000},{"id":"u_bob","email":"bob@example.com","password":"correct horse","display_name":"Bob","handle":"bob","balance":2500}],"payments":[{"id":"p_1","from_user_id":"u_ada","to_user_id":"u_bob","amount":500,"note":"coffee","visibility":"public"}],"requests":[{"id":"rq_1","requester_id":"u_bob","payer_id":"u_ada","amount":1200,"note":"taxi","status":"pending"}]}'
```

The specification says this reset returns `204` and requires no authentication.

```sh
curl -sS -D - http://127.0.0.1:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"correct horse"}'
```

The specification says login returns `200` and a JSON object with `user_id`, `display_name`, and `token`. Use that token as `<token>` below.

```sh
curl -sS -D - http://127.0.0.1:8080/payments \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <token>' \
  -H 'Idempotency-Key: 2f9c1a-walkthrough' \
  -d '{"to_handle":"bob","amount":1500,"note":"dinner","visibility":"public"}'
```

The specification says the first use of that key returns `201` with the payment: `amount` `1500`, `currency` `EUR`, `note` `dinner`, `visibility` `public`, `from_handle` `ada`, `to_handle` `bob`.

```sh
curl -sS -D - http://127.0.0.1:8080/me \
  -H 'Authorization: Bearer <token>'
```

The specification's `GET /me` object includes `handle`, `balance`, `currency`, and `minor_units`.
